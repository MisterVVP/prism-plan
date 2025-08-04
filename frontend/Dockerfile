FROM node:23-alpine AS builder
WORKDIR /app
ARG VITE_API_BASE_URL
ARG VITE_AUTH0_DOMAIN
ARG VITE_AUTH0_CLIENT_ID
ARG VITE_AUTH0_AUDIENCE

ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_AUTH0_DOMAIN=$VITE_AUTH0_DOMAIN
ENV VITE_AUTH0_CLIENT_ID=$VITE_AUTH0_CLIENT_ID
ENV VITE_AUTH0_AUDIENCE=$VITE_AUTH0_AUDIENCE

RUN test -n "$VITE_AUTH0_DOMAIN" && test -n "$VITE_AUTH0_CLIENT_ID" && test -n "$VITE_API_BASE_URL" && test -n "$VITE_AUTH0_AUDIENCE"
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# ---------- run stage ----------
FROM nginx:alpine
ENV CORS_ALLOWED_ORIGINS=.*
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 443
CMD ["nginx", "-g", "daemon off;"]
