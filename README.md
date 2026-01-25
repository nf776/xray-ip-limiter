# XRay IP Limiter

Система мониторинга и блокировки пользователей, превышающих лимит IP-адресов в VPN сети.

*Compatible with Remnawave*

## Описание

**XRay IP Limiter** - распределённая система для контроля подключений пользователей с разных IP-адресов. Если пользователь подключается с большего количества IP, чем разрешено (например, раздаёт доступ друзьям) - система автоматически его блокирует на всех нодах.

Легковесные агенты на каждой ноде отправляют информацию о подключениях на центральный сервер. Сервер анализирует данные со всех нод, находит нарушителей и рассылает команды блокировки обратно на все ноды.

## Архитектура
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Node 1    │     │   Node 2    │     │   Node N    │
│             │     │             │     │             │
│ access.log  │     │ access.log  │     │ access.log  │
│     ↓       │     │     ↓       │     │     ↓       │
│   Agent     │     │   Agent     │     │   Agent     │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                          │
                    ┌─────▼─────┐
                    │   NATS    │
                    └─────┬─────┘
                          │
                  ┌───────▼────────┐
                  │ Main Service   │
                  │                │
                  │ - Redis (IP)   │
                  │ - Processor    │
                  │                │
                  └───────┬────────┘
                          │
                    ┌─────▼─────┐
                    │   NATS    │
                    └─────┬─────┘
       ┌───────────────────┼───────────────────┐
       │                   │                   │
┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
│   Agent     │     │   Agent     │     │   Agent     │
│     ↓       │     │     ↓       │     │     ↓       │
│  iptables   │     │  iptables   │     │  iptables   │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Принцип работы

### 1. Сбор данных (Агенты)

На каждой ноде работает легковесный агент:
- Читает `access.log` в реальном времени
- Парсит IP-адреса и email пользователей
- Отправляет события в NATS JetStream

**Пример лога:**
```
2025/11/14 10:02:56 from 1.2.3.4:12345 accepted tcp:... email: 313
```

### 2. Анализ (Главный сервис)

Центральный сервер:
- Получает события от всех нод через NATS
- Сохраняет IP в Redis
- Считает уникальные IP для каждого пользователя
- Проверяет лимит (например, максимум 2 IP)

### 3. Блокировка (Команды)

Если лимит превышен:
- Формируется команда блокировки
- Отправляется в NATS на все ноды
- Агенты применяют iptables правила
- Пользователь заблокирован везде одновременно
- Блокировка происходит раз в 30 секунд (настраивается) и прекращается в тот момент, когда количество IP у пользователя входит в лимит

## Настройка

### Настройки главного сервера
```yaml
# config.yaml в папке с docker-compose главного сервера
nats:
  # НЕ МЕНЯТЬ ЕСЛИ СЕРВИС ЗАПУСКАЕТСЯ ЛОКАЛЬНО С NATS!
  url: "nats://nats:4222"

  # Установите токен в конфиге и docker compose, а так же на всех нодах
  token: "CHANGE_ME" 

service:
  ip_limit: 2
  ban_duration: "30s" # e.g. 30s 3h 1d
```

<details>
<summary>docker-compose.yaml</summary>

```yaml
services:
  nats:
    image: nats:latest
    container_name: "xray-ip-limiter-nats"
    ports:
      - "4222:4222"
    command: ["-js", "--auth", "CHANGE_ME"] # Установите токен из конфига вместо CHANGE_ME!
    networks:
      - observer
    volumes:
      - nats-data:/data

  redis:
    image: valkey/valkey:8.0
    container_name: xray-ip-limiter-redis
    restart: always
    volumes:
      - redis-data:/data
    networks:
      - observer
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  xray-ip-limiter:
    image: ghcr.io/nf776/xray-ip-limiter:latest
    container_name: xray-ip-limiter
    restart: always
    depends_on:
      redis:
        condition: service_healthy
      nats:
        condition: service_started
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    networks:
      - observer

networks:
  observer:
    driver: bridge

volumes:
  redis-data:
  nats-data:
```

</details>

### Настройка агента
```yaml
# config.yaml в папке с docker-compose агента
node_id: "node-us-4" # Установите уникальный ID ноды
log_path: "/var/log/remnanode/access.log" # ВНИМАНИЕ! При смене пути файла лога необходимо прокинуть новый путь в docker compose!

nats:
  url: "1.2.3.4:4222" # IP сервера, на котором запущен главный сервис. Порт NATS по умолчанию - 4222
  token: CHANGE_ME # Установите тот же токен, что вы ставили на главном сервисе
```

<details>
<summary>docker-compose.yaml на ноде</summary>

```
services:
  xray-ip-limiter-agent:
    image: ghcr.io/nf776/xray-ip-limiter-agent:latest
    container_name: xray-ip-limiter-agent
    restart: always
    network_mode: "host"
    cap_add:
      - NET_ADMIN
      - NET_RAW
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - /var/log/remnanode:/var/log/remnanode
```

</details>

## Быстрый старт

### Главный сервер

```bash
cd /opt && mkdir xray-ip-limiter && cd xray-ip-limiter
touch docker-compose.yaml config.yaml
nano docker-compose.yaml
nano config.yaml

docker compose up -d && docker compose logs -f
```

Настройте config.yaml и docker-compose.yaml согласно шаблонам выше

### Нода

```bash
cd /opt && mkdir xray-ip-limiter-agent && cd xray-ip-limiter-agent
touch docker-compose.yaml config.yaml
nano docker-compose.yaml
nano config.yaml

docker compose up -d && docker compose logs -f
```

Настройте config.yaml и docker-compose.yaml согласно шаблонам выше

## Производительность

### Пропускная способность

- **Агент:** до 10,000 строк лога/сек
- **Главный сервис:** до 100,000 событий/сек

## FAQ

**Q: Что если главный сервер упадет?**  
A: Агенты буферизуют события в памяти (200 штук). Старые события отбрасываются с Warning.

**Q: Что если пользователь сменил IP?**  
A: Старый IP автоматически удалится из через заданное время. По умолчанию - 30 секунд.

**Q: Можно ли изменить лимит на лету?**  
A: Да, отредактировать конфиг и перезапустить главный сервис.

## Roadmap

- [x] Парсинг логов (Agent)
- [x] Отправка в NATS (Agent)
- [x] Проверка лимитов (Main Service)
- [x] Redis кеширование
- [x] Отправка команд блокировки
- [x] IP Blocker (iptables)
- [ ] Telegram оповещения
- [ ] ClickHouse интеграция
- [ ] Prometheus метрики
- [ ] Web UI для управления
- [ ] API для внешних систем

## Лицензия

MIT

## Автор

Telegram: @nf776
