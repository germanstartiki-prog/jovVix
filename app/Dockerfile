FROM node:22-alpine

WORKDIR /app

RUN apk add --no-cache nginx supervisor wget curl

RUN mkdir -p /run/nginx

COPY ./package*.json ./
RUN npm install

COPY ./docker/nginx.conf /etc/nginx/nginx.conf
COPY ./docker/supervisiord.conf /etc/supervisord.conf

COPY . ./

RUN npm run build

EXPOSE 5000

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]

