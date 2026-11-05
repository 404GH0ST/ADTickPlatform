FROM python:3.15-slim

WORKDIR /app

COPY checker.py /app/checker

RUN chmod 755 /app/checker

ENTRYPOINT ["/app/checker"]
