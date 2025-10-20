export const connector = {
  name: {
    label: "Name",
    labelLocale: "rules.name",
    type: "text",
    placeholder: "",
    rules: [{ required: true, max: 20, trigger: "blur" }],
  },
  type: {
    type: "select",
    label: "Type",
    labelLocale: "rules.type",
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
    labelLocale: "rules.options",
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
      labelLocale: "rules.url",
      type: "text",
      placeholder: "amqp://guest:guest@test.com:5672/",
      rules: [{ required: true, max: 500 }],
    },
  },
  mqtt: {
    clientId: {
      label: "Client ID",
      labelLocale: "rules.clientId",
      type: "text",
      placeholder: "clientId",
      rules: [{ required: true }],
    },
    host: {
      label: "Host",
      labelLocale: "rules.host",
      type: "text",
      placeholder: "localhost",
      rules: [{ required: true }],
    },
    port: {
      label: "Port",
      labelLocale: "rules.port",
      type: "number",
      placeholder: "1883",
      rules: [{ required: true }],
    },
    user: {
      label: "User",
      labelLocale: "rules.user",
      type: "text",
      placeholder: "user",
      rules: [{ required: true }],
    },
    password: {
      label: "Password",
      labelLocale: "rules.password",
      type: "password",
      placeholder: "password",
      rules: [{ required: true }],
    },
    cleanSession: {
      label: "Clean Session",
      labelLocale: "rules.cleanSession",
      type: "checkbox",
    },
  },
  http: {
    url: {
      label: "URL",
      labelLocale: "rules.url",
      type: "text",
      placeholder: "http://localhost:8080",
      rules: [{ required: true }],
    },
    headers: {
      label: "Headers",
      labelLocale: "rules.headers",
      type: "kv",
    },
    timeout: {
      label: "Timeout(Second)",
      labelLocale: "rules.timeout",
      type: "number",
      placeholder: "10",
    },
  },
  influxdb: {
    // url, token, bucket, org, timePrecision(ns, us, ms, s), timeout
    url: { label: "URL", labelLocale: "rules.url", type: "text", placeholder: "http://localhost:8086", rules: [{ required: true }] },
    token: { label: "Token", labelLocale: "rules.token",  type: "text", placeholder: "token", rules: [{ required: true }] },
    bucket: { label: "Bucket", type: "text", placeholder: "bucket", rules: [{ required: true }] },
    org: { label: "Org", type: "text", placeholder: "org", rules: [{ required: true }] },
    timePrecision: {
      label: "Time Precision",
      labelLocale: "rules.timePrecision",
      type: "select",
      options: [
        { label: "Nanosecond ", value: "ns" },
        { label: "Microsecond", value: "us" },
        { label: "Millisecond", value: "ms" },
        { label: "Second", value: "s" },
      ],
      rules: [{ required: true }],
    },
    timeout: { label: "Timeout(Second)", labelLocale: "rules.timeout", type: "number", placeholder: "10" },
  },
  tdengine: {
    // url, username, password, timezone, database, timeout
    url: { label: "URL", type: "text", placeholder: "http://localhost:6030", rules: [{ required: true }] },
    username: { label: "Username", labelLocale: "rules.username", type: "text", placeholder: "admin", rules: [{ required: true }] },
    password: { label: "Password", labelLocale: "rules.password", type: "password", placeholder: "password", rules: [{ required: true }] },
    timezone: { label: "Timezone", labelLocale: "rules.timezone", type: "text", placeholder: "UTC" },
    database: { label: "Database", labelLocale: "rules.database", type: "text", placeholder: "my_database", rules: [{ required: true }] },
    timeout: { label: "Timeout(Second)", labelLocale: "rules.timeout", type: "number", placeholder: "2" },
  },
  mysql: {
    // host, port, user password, db, charset, timezone, maxIdleConns, maxOpenConns, connMaxLifetime
    host: { label: "Host", labelLocale: "rules.host", type: "text", placeholder: "localhost", rules: [{ required: true }] },
    port: { label: "Port", labelLocale: "rules.port", type: "number", placeholder: "3306", rules: [{ required: true }] },
    user: { label: "User", labelLocale: "rules.user", type: "text", placeholder: "root", rules: [{ required: true }] },
    password: { label: "Password", labelLocale: "rules.password", type: "password", placeholder: "password", rules: [{ required: true }] },
    db: { label: "Database", labelLocale: "rules.database", type: "text", placeholder: "database", rules: [{ required: true }] },
    charset: { label: "Charset", labelLocale: "rules.charset", type: "text", placeholder: "utf8" },
    timezone: { label: "Timezone", labelLocale: "rules.timezone", type: "text", placeholder: "Asia%2FShanghai", rules: [{ required: true }] },
    maxIdleConns: { label: "Max Idle Connections", labelLocale: "rules.maxIdleConns", type: "number", placeholder: "2" },
    maxOpenConns: { label: "Max Open Connections", labelLocale: "rules.maxOpenConns", type: "number", placeholder: "5" },
    connMaxLifetime: { label: "Connection Max Lifetime", labelLocale: "rules.connMaxLifetime", type: "number", placeholder: "300" },
  },
  redis: {
    url: {
      label: "URL",
      labelLocale: "rules.url",
      type: "text",
      placeholder: "redis://user:password@localhost:6789/3?dial_timeout=3&db=1&read_timeout=6s&max_retries=2",
      rules: [{ required: true, max: 500 }],
    },
  },
};

export const sink = {
  name: {
    label: "Name",
    labelLocale: "rules.name",  
    type: "text",
    placeholder: "",
    rules: [{ required: true }],
  },
  type: {
    label: "Type",
    labelLocale: "rules.type",
    type: "select",
    options: [
      { label: "Embed MQTT", labelLocale: "rules.embedMqtt", value: "embed-mqtt" },
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
    labelLocale: "rules.connector",
    type: "select",
    options: [],
    rules: [{ required: true }],
  },
  options: {
    label: "Options",
    labelLocale: "rules.options",
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
        { value: 0, label: "At most once", labelLocale: "rules.atMostOnce" },
        { value: 1, label: "At least once", labelLocale: "rules.atLeastOnce" },
        { value: 2, label: "Exactly once", labelLocale: "rules.exactlyOnce" },
      ],
    },
    retained: {
      label: "Retained",
      labelLocale: "rules.retained",
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
        { value: 0, label: "At most once", labelLocale: "rules.atMostOnce" },
        { value: 1, label: "At least once", labelLocale: "rules.atLeastOnce" },
        { value: 2, label: "Exactly once", labelLocale: "rules.exactlyOnce" },
      ],
    },
    retained: {
      label: "Retained",
      labelLocale: "rules.retained",
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
      labelLocale: "rules.method",
      type: "select",
      options: [
        { label: "POST", value: "POST" },
        { label: "PUT", value: "PUT" },
      ],
      rules: [{ required: true }],
    },
    path: {
      label: "Path",
      labelLocale: "rules.path",
      type: "text",
      placeholder: "/api/xxx",
      rules: [{ required: true }],
    },
    headers: {
      label: "Headers",
      labelLocale: "rules.headers",
      type: "kv",
    },
  },
  influxdb: {},
  mysql: {},
  redis: {},
  tdengine: {},
  log: {},
};

export const sinkTips = {
  influxdb: {
    note: `The format of the received data needs to be 
    <a href="https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/" target="_blank">line protocal<a>.`,
    labelLocale: "rules.influxdbNote",
    formatExamples: ["test,thingId=test v=1 1711529686403", "test,thingId=test,zone=east v=1,v2=3 1711529686403"],
  },
  tdengine: {
    note: `The format of the received data needs to be SQL string for 
      <a href="https://docs.tdengine.com/reference/connectors/rest-api/" target="_blank">REST API</a>.
      The table in TDengine should be created manually.`,
    labelLocale: "rules.tdengineNote",
    formatExamples: ['INSERT INTO presence(ts, thing_id, type) VALUES(1711529686403, "test", "connected")'],
  },
  mysql: {
    note: `The format of the received data needs to be SQL string for 
    <a href="https://dev.mysql.com/doc/refman/8.0/en/insert.html" target="_blank">INSERT</a>.
      The table in MySQL should be created manually.`,
    labelLocale: "rules.mysqlNote",
    formatExamples: [
      'INSERT INTO `data_latest` (`thing_id`, `name`, `time`, `value`, `type`) VALUES ("test", "temp", NOW(), "37", "number")',
    ],
  },
  redis: {
    note: `The format of the received data needs to be a 
    <a href="https://redis.io/docs/latest/commands/" target="_blank">redis command</a>.`,
    labelLocale: "rules.redisNote",
    formatExamples: ["HSET prop:test hum 50 temp 37"],
  },
};

export const defaultSink = {
  name: "",
  type: "embed-mqtt",
  options: {
    topic: "$iothub/user/things/${thingId}/republish",
  },
};

export const source = {
  name: {
    label: "Name",
    labelLocale: "rules.name",
    type: "text",
    placeholder: "",
    rules: [{ required: true, max: 20 }],
  },
  type: {
    label: "Type",
    labelLocale: "rules.type",
    type: "select",
    options: [
      { label: "Embed MQTT", value: "embed-mqtt", withoutConn: true },
      { label: "MQTT", value: "mqtt" },
    ],
    rules: [{ required: true, max: 20 }],
  },
  connector: {
    label: "Connector",
    labelLocale: "rules.connector",
    type: "select",
    options: [],
  },
  options: {
    label: "Options",
    labelLocale: "rules.options",
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
        { value: 0, label: "At most once", labelLocale: "rules.atMostOnce" },
        { value: 1, label: "At least once", labelLocale: "rules.atLeastOnce" },
        { value: 2, label: "Exactly once", labelLocale: "rules.exactlyOnce" },
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
