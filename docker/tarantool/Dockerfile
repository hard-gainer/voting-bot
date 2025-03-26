FROM tarantool/tarantool:latest

RUN apt-get update && apt-get install -y \
    procps \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/tarantool

RUN mkdir -p /var/log/tarantool && \
    chown -R tarantool:tarantool /var/log/tarantool && \
    chmod -R 755 /var/log/tarantool

COPY init.lua /opt/tarantool/

RUN chmod +x /opt/tarantool/init.lua

EXPOSE 3301

CMD ["tarantool", "/opt/tarantool/init.lua"]