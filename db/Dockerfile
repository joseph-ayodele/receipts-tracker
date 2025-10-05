FROM postgres:latest
ENV PGDATA=/var/lib/postgresql/pgdata
RUN mkdir -p "$PGDATA" && chown -R 999:999 /var/lib/postgresql
COPY sql/*.sql /docker-entrypoint-initdb.d/
