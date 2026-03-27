FROM node:22-alpine

WORKDIR /app

COPY apps/web/.next/standalone ./
COPY apps/web/.next/static ./apps/web/.next/static
COPY apps/web/public ./apps/web/public
COPY docs /app/docs

WORKDIR /app/apps/web

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

CMD ["node", "server.js"]
