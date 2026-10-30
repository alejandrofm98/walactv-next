FROM node:22.23.4-alpine AS build

WORKDIR /src
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build -- --configuration production

FROM nginx:1.29-alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist/walactv-web/browser/ /usr/share/nginx/html/
EXPOSE 80
