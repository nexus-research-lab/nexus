#!/bin/sh
set -eu

: "${NGINX_SERVER_NAME:=_}"
: "${NGINX_SSL_CERTIFICATE:=/etc/nginx/certs/fullchain.pem}"
: "${NGINX_SSL_CERTIFICATE_KEY:=/etc/nginx/certs/privkey.pem}"
: "${NGINX_REDIRECT_HTTPS:=true}"
export NGINX_SERVER_NAME NGINX_SSL_CERTIFICATE NGINX_SSL_CERTIFICATE_KEY

template_dir=/etc/nginx/nexus-templates
vars='${NGINX_SERVER_NAME} ${NGINX_SSL_CERTIFICATE} ${NGINX_SSL_CERTIFICATE_KEY}'

mkdir -p /etc/nginx/conf.d
rm -f /etc/nginx/conf.d/default.conf /etc/nginx/conf.d/nexus-http.conf /etc/nginx/conf.d/nexus-https.conf

if [ -s "${NGINX_SSL_CERTIFICATE}" ] && [ -s "${NGINX_SSL_CERTIFICATE_KEY}" ]; then
    case "$(printf '%s' "${NGINX_REDIRECT_HTTPS}" | tr '[:upper:]' '[:lower:]')" in
        1|true|yes|on)
            envsubst "${vars}" < "${template_dir}/http-redirect.conf.template" > /etc/nginx/conf.d/nexus-http.conf
            ;;
        *)
            envsubst "${vars}" < "${template_dir}/http.conf.template" > /etc/nginx/conf.d/nexus-http.conf
            ;;
    esac
    envsubst "${vars}" < "${template_dir}/https.conf.template" > /etc/nginx/conf.d/nexus-https.conf
else
    # 中文注释：没有证书时只生成 HTTP 配置，保证开源默认部署不依赖私有证书。
    envsubst "${vars}" < "${template_dir}/http.conf.template" > /etc/nginx/conf.d/nexus-http.conf
fi
