FROM node:23-alpine AS builder
WORKDIR /app
ARG VITE_AUTH0_DOMAIN
ARG VITE_AUTH0_CLIENT_ID
ARG VITE_AUTH0_AUDIENCE
ARG VITE_STREAM_URL

ENV VITE_AUTH0_DOMAIN=$VITE_AUTH0_DOMAIN
ENV VITE_AUTH0_CLIENT_ID=$VITE_AUTH0_CLIENT_ID
ENV VITE_AUTH0_AUDIENCE=$VITE_AUTH0_AUDIENCE
ENV VITE_STREAM_URL=$VITE_STREAM_URL

RUN test -n "$VITE_AUTH0_DOMAIN" && test -n "$VITE_AUTH0_CLIENT_ID" && test -n "$VITE_AUTH0_AUDIENCE"
COPY package*.json ./
RUN npm install --ignore-scripts
COPY . .
RUN npm run build

# ---------- run stage ----------
FROM nginx:alpine
ENV CORS_ALLOWED_ORIGINS=.*
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
