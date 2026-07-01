# Dev image: runs the Vite dev server with HMR. Source is bind-mounted in
# docker-compose, with node_modules kept in a container-owned volume so the
# host directory doesn't shadow the installed dependencies.
FROM node:24-alpine
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .

EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
