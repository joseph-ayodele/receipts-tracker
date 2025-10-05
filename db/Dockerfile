FROM postgres:latest
ENV PGDATA=/var/lib/postgresql/pgdata
RUN mkdir -p "$PGDATA" && chown -R 999:999 /var/lib/postgresql
COPY migrations/001_init.sql /docker-entrypoint-initdb.d/
