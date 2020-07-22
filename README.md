<img src="logo.png" width="300px">

### A cloud-native, distributed, master-less, secrets-capable key-value database

# What is CAVE?
Cave is a reaction to the technologies on the market in both COTS and OSS worlds. We identified a need to have a fast, reliable, cloud-native key-value database that doesn't require a lot of configuration or workarounds to make a master/worker model...work. 

# What is it not?
Cave is not ACID compliant and makes no write guarantees. 

# How does it work?
Cave is based on a DHT network. Cluster peers are discovered by sharing lists of peers with other peers, this means that a node only needs to "see" a single node of a cluster in order to fully join and connect with the rest of the cluster. 
All updates (writes and deletes) to the database are broadcast across the network, each peer listens for updates and applies them as they come in. 

# Going Deeper

### Building
Cave can be built by running `make build`

### Configuration
Configuration happens one of three ways:
* Config file
* Environment variables
* Command-line arguments

Command-line arguments take precedence over all other methods. 
You can get a full list of configuration parameters by running `cave --help`

### Running
To start Cave in single-node development mode, simply run `cave --mode=dev`. This will start a new single-node database on your local machine.

To start Cave in "production" mode, you must supply the `--mode=prod` flag, otherwise it will default to single-node "development" mode. When running in "production" mode, the new database instance will attempt to discover peers and sync the cluster database state. If it is unable to find peers it will assume it is the first node to come up and generate a new cluster id, shared keys, and other items.

### Monitoring
Cave comes with a ton of exported Prometheus metrics. They can be scraped at the `/api/v1/perf/metrics` endpoint.

### Interacting with Cave
Cave can be used via the REST API. Full API spec will be provided below. In general, there are a few things to remember:

* All API requests are done with the `/api/v1/` prefix.
* When reading or writing a secret, you must supply the `secret=true` URL parameter in order to encrypt/decrypt the secret


# API

## KV

### /api/v1/kv/[path/.../path]/keyname
```
Methods: GET, POST, DELETE
GET - Getting a path and key name will read that path and key name from the db
POST - POSTing data to a path and key name will store data at that path and key name
DELETE - DELETE will delete a key and value at a given path name
```

## PERF

### /api/v1/perf/logs
```
Methods: GET
Endpoint to get node logs (if enabled)
```

### /api/v1/perf/metrics
```
Methods: GET
Prometheus endpoint
```

### /api/v1/perf/dashboard
```
Methods: GET
Returns JSON configuration for a Cave-specific Grafana dashboard
```

## SYSTEM

### /api/v1/system/config
```
Methods: GET
Returns system configuration as JSON
```

### /api/v1/system/info
```Methods: GET
Returns system information as JSON
```


# Web UI
Cave has a _very_ rudimentary web UI that allows you to browse the key-value store and see which nodes are active. 
The UI can be accessed by going to
```
https://cave_host:port/ui/
```

# Roadmap
Cave is very much a work in progress. Please bear with us as we work to improve it. Our proposed development roadmap is as follows:

* Implement plug-in system with YetiCloud Airboss
* Migrate cluster communication to JSON-RPC
* Periodic re-sync between nodes
* API
  * DB export
  * mgmt API
  * auth api
* Testing
* Enforce key locking
* cloud discovery (AWS, GCP, Azure)
* audit trail logging

# Contributing
Contributors are welcome! Please be respectful of the source and the other contributors. 
