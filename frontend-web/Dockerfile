# Frontend dev server — hot-reload via bind mount in docker-compose
FROM node:20-alpine

WORKDIR /app

# Install dependencies first for better layer caching.
# In docker-compose, node_modules lives inside the container to avoid
# host platform conflicts; the source tree is bind-mounted at /app.
COPY package.json package-lock.json ./
RUN npm ci

EXPOSE 3000

CMD ["npm", "run", "dev"]
