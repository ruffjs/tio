// Connector options type definitions
interface AmqpOptions {
  url: string;
}

interface MqttOptions {
  clientId: string;
  host: string;
  port: number;
  user: string;
  password: string;
  cleanSession: boolean;
}

type ConnectorOptions = AmqpOptions | MqttOptions;

// Connector type definition
interface Connector {
  name: string;
  type: 'amqp' | 'mqtt';
  options: ConnectorOptions;
}

// Source options type definitions
interface SourceOptions {
  topic: string;
  qos: number;
}

// Source type definition
interface Source {
  name: string;
  type: string; // e.g., 'embed-mqtt' or 'mqtt'
  connector?: string; // Used only if the source type is 'mqtt'
  options: SourceOptions;
}

// Sink options type definitions
interface AmqpSinkOptions {
  exchange: string;
  routingKey?: string; // Optional
}

interface MqttSinkOptions {
  topic: string;
}

type SinkOptions = AmqpSinkOptions | MqttSinkOptions;

// Sink type definition
export interface Sink {
  name: string;
  type: 'amqp' | 'mqtt';
  connector: string;
  options: SinkOptions;
}

export interface Process {
  type: 'transform' | 'filter';
  name: string;
  runner: 'js' | 'jq';
  jq?: string; // Optional, used when runner js 'jq'
  js?: string; // Optional, used when runner is 'js'
}

// Rule type definition
export interface Rule {
  name: string;
  note: string;
  sources: string[];
  process: Process[];
  sinks: string[];
}

// Root object type definition
export interface RuleConfig {
  connectors: Connector[];
  sources: Source[];
  sinks: Sink[];
  rules: Rule[];
}


export interface RuleEditData {
  name: string;
  note: string;
  enabled: boolean;
  sources: Source[];
  process: Process[];
  sinks: Sink[];
}

export interface RuleValidateResult {
  isValid: boolean;
  errors: string[];
  sourcesShared: string[];
  sinksShared: string[];
  config: RuleConfig;
}
