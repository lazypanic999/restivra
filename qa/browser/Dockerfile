FROM mcr.microsoft.com/playwright:v1.64.1-noble
WORKDIR /tests
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY smoke.mjs ./
ENTRYPOINT ["node", "/tests/smoke.mjs"]
