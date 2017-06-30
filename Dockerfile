FROM nginx:1.13.1-alpine

# forward nginx access and error logs to stdout and stderr of the ingress
# controller process
RUN ln -sf /proc/1/fd/1 /var/log/nginx/access.log \
	&& ln -sf /proc/1/fd/2 /var/log/nginx/error.log

COPY bin/nginx-ingress pkg/nginx/ingress.tmpl pkg/nginx/nginx.conf.tmpl /

RUN rm /etc/nginx/conf.d/*

ENTRYPOINT ["/nginx-ingress"]
