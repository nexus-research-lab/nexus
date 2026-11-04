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

(cd "${repo_root}" && go run ./scripts/check-architecture)

base_commit="$(resolve_base_commit)"
changed_list_path="$(mktemp "${TMPDIR:-/tmp}/nexus-go-changed.XXXXXX")"
package_list_path="$(mktemp "${TMPDIR:-/tmp}/nexus-go-packages.XXXXXX")"
trap 'rm -f "${changed_list_path}" "${package_list_path}"' EXIT

{
	git -C "${repo_root}" diff --name-only --no-renames --diff-filter=ACDMRT "${base_commit}...HEAD"
	git -C "${repo_root}" diff --name-only --no-renames --diff-filter=ACDMRT
	git -C "${repo_root}" diff --cached --name-only --no-renames --diff-filter=ACDMRT
	git -C "${repo_root}" ls-files --others --exclude-standard
} | sort -u >"${changed_list_path}"

# trimcheck is a separate module and is absent from the root go list graph.
if grep -q '^tools/trimcheck/' "${changed_list_path}"; then
	(
		cd "${repo_root}/tools/trimcheck"
		export GOWORK=off
		go vet -p="${package_parallelism}" ./...
		module_test_args=(-vet=off -p="${package_parallelism}")
		if [[ ${fresh_mode} -eq 1 ]]; then module_test_args+=(-count=1); fi
		go test "${module_test_args[@]}" ./...
	)
fi

# A failed graph load must fail the gate, not silently produce an empty scope.
(cd "${repo_root}" && go run ./scripts/check-go-scope <"${changed_list_path}") >"${package_list_path}"

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
