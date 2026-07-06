# Mock service image — a plain Node runtime; no dev server / HMR needed since
# these are throwaway stand-ins that rarely change.
FROM node:24-alpine
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install --omit=dev

COPY . .

EXPOSE 8080
CMD ["node", "server.js"]
