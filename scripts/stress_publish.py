#!/usr/bin/env python3
import paho.mqtt.client as mqtt
import time
import json
import threading
import sys
import argparse

# Configuration
MQTT_HOST = "localhost"
MQTT_PORT = 1883
TOPIC = "test_queue"
USER = "$tio"
PASSWORD = "public"

def run_client(client_id, msg_count, qos, interval):
    # Handle paho-mqtt 2.0+ callback API version
    try:
        client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, client_id=client_id)
    except AttributeError:
        client = mqtt.Client(client_id)
    
    client.username_pw_set(USER, PASSWORD)
    
    # Track published messages
    publish_infos = []
    
    try:
        client.connect(MQTT_HOST, MQTT_PORT, 60)
        client.loop_start() # Start background thread to handle ACKs
        
        for i in range(1, msg_count + 1):
            payload = {
                "ts": int(time.time() * 1000),
                "sender": client_id,
                "seq": i
            }
            info = client.publish(TOPIC, json.dumps(payload), qos=qos)
            publish_infos.append(info)
            
            if i % 1000 == 0:
                print(f"Client {client_id} sent {i} messages")
            
            if interval > 0:
                time.sleep(interval)
        
        # If QoS > 0, wait for the last message to be acknowledged
        if qos > 0 and publish_infos:
            print(f"Client {client_id} waiting for server ACKs...")
            publish_infos[-1].wait_for_publish()
                
        print(f"Client {client_id} finished.")
        client.loop_stop()
        client.disconnect()
    except Exception as e:
        print(f"Client {client_id} error: {e}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='MQTT Stress Publisher')
    parser.add_argument('-cl', type=int, default=10, help='Number of concurrent clients')
    parser.add_argument('-c', type=int, default=1000, help='Messages per client')
    parser.add_argument('-q', type=int, default=0, choices=[0, 1, 2], help='MQTT QoS level')
    parser.add_argument('-i', type=float, default=0.001, help='Interval between messages in seconds')
    
    args = parser.parse_args()
    
    threads = []
    start_time = time.time()
    
    print(f"Starting {args.cl} clients (QoS {args.q}) to send {args.c} messages each...")
    
    for i in range(1, args.cl + 1):
        client_id = f"python-sender-{i}"
        t = threading.Thread(target=run_client, args=(client_id, args.c, args.q, args.i))
        t.start()
        threads.append(t)
    
    for t in threads:
        t.join()
        
    duration = time.time() - start_time
    total_msgs = args.cl * args.c
    print(f"\nSummary:")
    print(f"Total messages sent: {total_msgs}")
    print(f"Total time: {duration:.2f} seconds")
    print(f"Throughput: {total_msgs / duration:.2f} msg/s")
