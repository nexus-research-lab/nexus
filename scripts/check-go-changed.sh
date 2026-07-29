#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
package_parallelism="${GO_TEST_PACKAGE_PARALLELISM:-4}"
fresh_mode=0

if [[ "${1:-}" == "--fresh" ]]; then
	fresh_mode=1
elif [[ $# -gt 0 ]]; then
	echo "usage: $0 [--fresh]" >&2
	exit 2
fi

resolve_base_commit() {
	local requested_ref="${GO_CHECK_BASE_REF:-}"
	if [[ -n "${requested_ref}" ]]; then
		git -C "${repo_root}" merge-base HEAD "${requested_ref}" 2>/dev/null ||
			git -C "${repo_root}" rev-parse "${requested_ref}"
		return
	fi
	if git -C "${repo_root}" rev-parse --verify '@{upstream}' >/dev/null 2>&1; then
		git -C "${repo_root}" merge-base HEAD '@{upstream}'
		return
	fi
	if git -C "${repo_root}" rev-parse --verify HEAD^ >/dev/null 2>&1; then
		git -C "${repo_root}" rev-parse HEAD^
		return
	fi
	git -C "${repo_root}" rev-parse HEAD
}

append_nearest_go_package() {
	local changed_path="$1"
	local package_dir
	local go_files

	package_dir="$(dirname "${changed_path}")"
	while [[ "${package_dir}" != "." && "${package_dir}" != "/" ]]; do
		go_files=("${repo_root}/${package_dir}"/*.go)
		if [[ -e "${go_files[0]}" ]]; then
			printf './%s\n' "${package_dir}" >>"${package_list_path}"
			return
		fi
		package_dir="$(dirname "${package_dir}")"
	done

	# 整包删除后无法从当前目录恢复归属，改跑全量避免漏掉引用方。
	if [[ "${changed_path}" == *.go ]]; then
		printf './...\n' >>"${package_list_path}"
	fi
}

base_commit="$(resolve_base_commit)"
changed_list_path="$(mktemp "${TMPDIR:-/tmp}/nexus-go-changed.XXXXXX")"
package_list_path="$(mktemp "${TMPDIR:-/tmp}/nexus-go-packages.XXXXXX")"
trap 'rm -f "${changed_list_path}" "${package_list_path}"' EXIT

{
	git -C "${repo_root}" diff --name-only --diff-filter=ACMR "${base_commit}...HEAD"
	git -C "${repo_root}" diff --name-only --diff-filter=ACMR
	git -C "${repo_root}" diff --cached --name-only --diff-filter=ACMR
	git -C "${repo_root}" ls-files --others --exclude-standard
} | sort -u >"${changed_list_path}"

while IFS= read -r changed_path; do
	[[ -n "${changed_path}" ]] || continue
	case "${changed_path}" in
	go.mod | go.sum)
		printf './...\n' >>"${package_list_path}"
		;;
	skills/*)
		printf '%s\n' \
			'./internal/service/skills' \
			'./internal/service/workspace' \
			'./internal/handler/workspace' >>"${package_list_path}"
		;;
	cmd/* | internal/* | tools/*)
		append_nearest_go_package "${changed_path}"
		;;
	esac
done <"${changed_list_path}"

packages=()
while IFS= read -r package_path; do
	[[ -n "${package_path}" ]] && packages+=("${package_path}")
done < <(sort -u "${package_list_path}")

if [[ ${#packages[@]} -eq 0 ]]; then
	echo "No Go-relevant changes; skipping Go checks."
	exit 0
fi

if [[ " ${packages[*]} " == *" ./... "* ]]; then
	packages=("./...")
fi

echo "Go check scope (${#packages[@]} package paths, base ${base_commit:0:12}):"
printf '  %s\n' "${packages[@]}"

cd "${repo_root}"
go vet -p="${package_parallelism}" "${packages[@]}"

test_args=(-vet=off -p="${package_parallelism}")
if [[ ${fresh_mode} -eq 1 ]]; then
	test_args+=(-count=1)
fi
go test "${test_args[@]}" "${packages[@]}"
