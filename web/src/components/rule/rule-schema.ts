export const connector = {
  name: {
    label: "Name",
    type: "text",
    placeholder: "",
    rules: [{ required: true, max: 20, trigger: "blur" }],
  },
  type: {
    type: "select",
    label: "Type",
    options: [
      { label: "AMQP", value: "amqp" },
      { label: "MQTT", value: "mqtt" },
      { label: "HTTP", value: "http" },
      { label: "InfluDB", value: "influxdb" },
      { label: "MySQL", value: "mysql" },
      { label: "Redis", value: "redis" },
      { label: "TDengine", value: "tdengine" },
    ],
    rules: [{ required: true }],
  },
  options: {
    type: "object",
    label: "Options",
    items: {},
  },
};
export const defaultConnector = {
  name: "",
  type: "amqp",
  options: {
    url: "amqp://guest:guest@test.com:5672/",
  },
};

export const connectorOptions = {
  amqp: {
    url: {
      label: "URL",
      type: "text",
      placeholder: "amqp://guest:guest@test.com:5672/",
      rules: [{ required: true, max: 500 }],
    },
  },
  mqtt: {
    clientId: {
      label: "Client ID",
      type: "text",
      placeholder: "clientId",
      rules: [{ required: true }],
    },
    host: {
      label: "Host",
      type: "text",
      placeholder: "localhost",
      rules: [{ required: true }],
    },
    port: {
      label: "Port",
      type: "number",
      placeholder: "1883",
      rules: [{ required: true }],
    },
    user: {
      label: "User",
      type: "text",
      placeholder: "user",
      rules: [{ required: true }],
    },
    password: {
      label: "Password",
      type: "password",
      placeholder: "password",
      rules: [{ required: true }],
    },
    cleanSession: {
      label: "Clean Session",
      type: "checkbox",
    },
  },
  http: {
    url: {
      label: "URL",
      type: "text",
      placeholder: "http://localhost:8080",
      rules: [{ required: true }],
    },
    headers: {
      label: "Headers",
      type: "kv",
    },
    timeout: {
      label: "Timeout(Second)",
      type: "number",
      placeholder: "10",
    },
  },
  influxdb: {
    // url, token, bucket, org, timePrecision(ns, us, ms, s), timeout
    url: { label: "URL", type: "text", placeholder: "http://localhost:8086", rules: [{ required: true }] },
    token: { label: "Token", type: "text", placeholder: "token", rules: [{ required: true }] },
    bucket: { label: "Bucket", type: "text", placeholder: "bucket", rules: [{ required: true }] },
    org: { label: "Org", type: "text", placeholder: "org", rules: [{ required: true }] },
    timePrecision: {
      label: "Time Precision",
      type: "select",
      options: [
        { label: "Nanosecond ", value: "ns" },
        { label: "Microsecond", value: "us" },
        { label: "Millisecond", value: "ms" },
        { label: "Second", value: "s" },
      ],
      rules: [{ required: true }],
    },
    timeout: { label: "Timeout(Second)", type: "number", placeholder: "10" },
  },
  tdengine: {
    // url, username, password, timezone, database, timeout
    url: { label: "URL", type: "text", placeholder: "tdengine://localhost:6030/rest/sql", rules: [{ required: true }] },
    username: { label: "Username", type: "text", placeholder: "admin", rules: [{ required: true }] },
    password: { label: "Password", type: "password", placeholder: "password", rules: [{ required: true }] },
    timezone: { label: "Timezone", type: "text", placeholder: "UTC" },
    database: { label: "Database", type: "text", placeholder: "my_database", rules: [{ required: true }] },
    timeout: { label: "Timeout(Second)", type: "number", placeholder: "10" },
  },
  mysql: {
    // host, port, user password, db, charset, timezone, maxIdleConns, maxOpenConns, connMaxLifetime
    host: { label: "Host", type: "text", placeholder: "localhost", rules: [{ required: true }] },
    port: { label: "Port", type: "number", placeholder: "3306", rules: [{ required: true }] },
    user: { label: "User", type: "text", placeholder: "root", rules: [{ required: true }] },
    password: { label: "Password", type: "password", placeholder: "password", rules: [{ required: true }] },
    db: { label: "Database", type: "text", placeholder: "database", rules: [{ required: true }] },
    charset: { label: "Charset", type: "text", placeholder: "utf8" },
    timezone: { label: "Timezone", type: "text", placeholder: "Asia%2FShanghai", rules: [{ required: true }] },
    maxIdleConns: { label: "Max Idle Connections", type: "number", placeholder: "2" },
    maxOpenConns: { label: "Max Open Connections", type: "number", placeholder: "5" },
    connMaxLifetime: { label: "Connection Max Lifetime", type: "number", placeholder: "300" },
  },
  redis: {
    url: {
      label: "URL",
      type: "text",
      placeholder: "redis://user:password@localhost:6789/3?dial_timeout=3&db=1&read_timeout=6s&max_retries=2",
      rules: [{ required: true, max: 500 }],
    },
  },
};

export const sink = {
  name: {
    label: "Name",
    type: "text",
    placeholder: "",
    rules: [{ required: true }],
  },
  type: {
    label: "Type",
    type: "select",
    options: [
      { label: "Embed MQTT", value: "embed-mqtt" },
      { label: "MQTT", value: "mqtt" },
      { label: "AMQP", value: "amqp" },
      { label: "HTTP", value: "http" },
      { label: "InfluxDB v2", value: "influxdb" },
      { label: "TDengine", value: "tdengine" },
      { label: "MySQL", value: "mysql" },
      { label: "Redis", value: "redis" },
      // TODO add Log sink
    ],
    rules: [{ required: true }],
  },
  connector: {
    label: "Connector",
    type: "select",
    options: [],
    rules: [{ required: true }],
  },
  options: {
    label: "Options",
    type: "object",
    items: {},
  },
};

export const sinkOptions = {
  mqtt: {
    topic: {
      label: "Topic",
      type: "text",
      placeholder: "topic",
      rules: [{ required: true }],
    },
    qos: {
      label: "QoS",
      type: "select",
      options: [
        { value: 0, label: "At most once" },
        { value: 1, label: "At least once" },
        { value: 2, label: "Exactly once" },
      ],
    },
    retained: {
      label: "Retained",
      type: "checkbox",
    },
  },
  "embed-mqtt": {
    topic: {
      label: "Topic",
      type: "text",
      placeholder: "topic",
      rules: [{ required: true }],
    },
    qos: {
      label: "QoS",
      type: "select",
      options: [
        { value: 0, label: "At most once" },
        { value: 1, label: "At least once" },
        { value: 2, label: "Exactly once" },
      ],
    },
    retained: {
      label: "Retained",
      type: "checkbox",
    },
  },
  amqp: {
    exchange: {
      label: "Exchange",
      type: "text",
      placeholder: "exchange",
      rules: [{ required: true }],
    },
    routingKey: {
      label: "Routing Key",
      type: "text",
    },
  },
  http: {
    method: {
      label: "Method",
      type: "select",
      options: [
        { label: "POST", value: "POST" },
        { label: "PUT", value: "PUT" },
      ],
      rules: [{ required: true }],
    },
    path: {
      label: "Path",
      type: "text",
      placeholder: "/api/xxx",
      rules: [{ required: true }],
    },
    headers: {
      label: "Headers",
      type: "kv",
    },
  },
  influxdb: {},
  mysql: {},
  redis: {},
  tdengine: {},
  log: {},
};

export const defaultSink = {
  name: "",
  type: "embed-mqtt",
  options: {
    topic: "$iothub/user/things/{thingId}/republish",
  },
};

export const source = {
  name: {
    label: "Name",
    type: "text",
    placeholder: "",
    rules: [{ required: true, max: 20 }],
  },
  type: {
    label: "Type",
    type: "select",
    options: [
      { label: "Embed MQTT", value: "embed-mqtt", withoutConn: true },
      { label: "MQTT", value: "mqtt" },
    ],
    rules: [{ required: true, max: 20 }],
  },
  connector: {
    label: "Connector",
    type: "select",
    options: [],
  },
  options: {
    label: "Options",
    type: "object",
    items: {},
  },
};

export const sourceOptions = {
  "embed-mqtt": {
    topic: { label: "Topic", name: "topic", type: "text", placeholder: "topic", rules: [{ required: true }] },
  },
  mqtt: {
    topic: { label: "Topic", type: "text", placeholder: "topic", rules: [{ required: true }] },
    qos: {
      label: "QoS",
      type: "select",
      placeholder: "QoS",
      options: [
        { value: 0, label: "At most once" },
        { value: 1, label: "At least once" },
        { value: 2, label: "Exactly once" },
      ],
    },
  },
};

export const defaultSrc = {
  name: "",
  type: "embed-mqtt",
  options: {
    topic: "$iothub/",
  },
};
