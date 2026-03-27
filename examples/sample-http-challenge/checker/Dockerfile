FROM python:3.12-slim

WORKDIR /app

COPY checker.py /app/checker

RUN chmod 755 /app/checker

ENTRYPOINT ["/app/checker"]
