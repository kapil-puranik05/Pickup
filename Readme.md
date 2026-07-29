# Distributed Object Storage System

This project is a Go-based prototype of a distributed object storage system built around chunked file storage, chain replication, and centralized metadata management.

At a high level, the system lets a client upload, retrieve, and delete files by splitting them into chunks, distributing those chunks through a storage chain, and tracking object metadata through a router service backed by PostgreSQL.

## What This Project Is About

The repository explores how a distributed storage system can be organized using:

- A client that talks to the control plane and storage nodes
- A router that manages metadata and exposes the public API
- A master service that manages storage node membership and chain layout
- Storage nodes that replicate chunks across a chain and serve reads from the tail

The design reflects core distributed systems ideas such as:

- Chunk-based object storage
- Chain replication
- Epoch-based reconfiguration
- Heartbeat-driven failure detection
- Separation of metadata from data path operations

## Architecture Overview

The system is split into three main modules:

### 1. Client

The client is responsible for initiating file operations:

- Upload a file
- Retrieve a file
- Delete a file

For uploads, the client:

1. Sends file metadata to the router
2. Receives an object ID and active chain topology
3. Fetches the current epoch from each chain master
4. Splits the file into 64 MB chunks
5. Sends chunks to the head of the selected chain
6. Notifies the router after upload completion

For retrieval, the client:

1. Asks the router for object metadata and chain topology
2. Requests chunks from chain tails
3. Reassembles the chunks in order into the original file

For delete, the client:

1. Asks the router for the object ID and topology
2. Sends delete commands into the chain
3. Notifies the router to remove metadata after storage cleanup

### 2. Router

The router acts as the metadata and coordination layer exposed on `localhost:8000`.

Its responsibilities include:

- Registering active storage chains
- Creating object metadata on upload initialization
- Marking uploads complete
- Returning retrieval metadata
- Returning delete metadata
- Removing object metadata after delete completion

Object metadata is stored in PostgreSQL using GORM. Each object record contains:

- `ID`
- `Key`
- `Size`
- `ChunkSize`
- `Status`
- Timestamps

The object status lifecycle currently includes:

- `UPLOADING`
- `READY`
- `DELETED`

## 3. Storage Subsystem

The storage subsystem is made of:

- One master process per chain
- Multiple storage node processes

### Master

The master is responsible for:

- Registering nodes as they join
- Maintaining the active chain layout
- Assigning node roles
- Tracking the current epoch
- Detecting node failures using heartbeats
- Reconfiguring the chain when membership changes
- Registering the active chain with the router

The master exposes endpoints for:

- Node registration
- Layout queries
- Heartbeats

### Storage Nodes

Each node:

- Loads its local configuration from `node.json`
- Registers with the chain master
- Accepts reconfiguration commands
- Stores chunks on local disk
- Replicates writes to the next node in the chain
- Sends acknowledgements back upstream
- Serves reads from local chunk directories

Roles assigned by the master:

- `HEAD`
- `MIDDLE`
- `TAIL`
- `ORPHAN`

### Chain Replication Model

Writes enter through the head node and flow downstream node by node until they reach the tail. Once the tail persists the data, it sends an acknowledgement upstream. Intermediate nodes maintain a local transit log so in-flight operations can be tracked until acknowledged.

Reads are served from the tail node. This matches the chain-replication idea that the tail represents the most up-to-date committed state.

Deletes reuse the same replication path by sending a `DELETE` command instead of a chunk write.

## Request Flow Summary

### Upload Flow

1. Client sends upload initialization request to router
2. Router creates metadata with `UPLOADING` status
3. Router returns object ID and registered chains
4. Client fetches current epoch from each chain master
5. Client sends chunks to chain heads
6. Nodes replicate chunks down the chain
7. Tail sends ACKs upstream
8. Client notifies router that upload is complete
9. Router marks the object as `READY`

### Retrieval Flow

1. Client sends retrieval initialization request to router
2. Router returns object ID, topology, and chunk count
3. Client requests chunk streams from chain tails
4. Client stores temporary chunk files
5. Client sorts chunks by chunk ID
6. Client reconstructs the original file

### Delete Flow

1. Client sends delete initialization request to router
2. Router returns object ID and topology
3. Client fetches current epoch from each chain master
4. Client sends `DELETE` commands to chain heads
5. Nodes delete local object directories and replicate the delete
6. Client notifies router after storage deletion completes
7. Router removes the object metadata

## Repository Structure

```text
Distributed Object Storage System/
├── client/
│   ├── go.mod
│   └── internal/
├── router/
│   ├── cmd/
│   ├── internal/
│   │   ├── database/
│   │   ├── handlers/
│   │   ├── metadata/
│   │   └── repositories/
│   └── go.mod
├── storage/
│   ├── cmd/
│   │   ├── master/
│   │   └── node/
│   ├── internal/
│   │   ├── master/
│   │   ├── node/
│   │   └── shared/
│   ├── master1/
│   ├── node1/
│   ├── node2/
│   ├── node3/
│   ├── cluster_start.ps1
│   ├── cluster_start.sh
│   └── go.mod
├── Architecture.png
├── Readme.md
└── Test Checklist.md
```

## Configuration

### Router

The router expects database configuration through environment variables:

- `DB_HOST`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `DB_PORT`

### Master

The storage master reads its configuration from:

- `MASTER_PATH`

Example value used by the provided PowerShell startup script:

- `./master1`

### Nodes

Each storage node reads its configuration from:

- `NODE_PATH`

Example values used by the provided PowerShell startup scripts:

- `./node1`
- `./node2`
- `./node3`

Each node configuration file includes:

- Node address
- Master address
- Node ID

## Running the Project

### Router

Start the router after setting the PostgreSQL environment variables:

```powershell
cd router
go run ./cmd
```

### Storage Cluster on Windows

The repository includes a helper script:

```powershell
cd storage
.\cluster_start.ps1
```

This script launches:

- One master process
- Three storage node processes

### Storage Cluster on Linux

The repository also includes:

```bash
cd storage
./cluster_start.sh
```

### Client

Run the client from the `client` module:

```powershell
cd client
go run .
```

Note: the current client entrypoint is set up for manual testing and currently invokes delete behavior by default, while upload and retrieval calls are present in code but commented for interactive switching.

## Storage Layout on Disk

Each storage node stores object data locally in a per-node directory structure. Chunks are written under folders named by object ID, with each chunk stored as a file named by its chunk index.

This means the local disk layout conceptually looks like:

```text
node1/
└── <object-id>/
    ├── 0
    ├── 1
    └── 2
```

Nodes also maintain a `log.txt` file that records forwarded write operations until they are acknowledged.

## Reliability Features

The implementation includes several important distributed system mechanisms:

- Periodic heartbeats from nodes to master
- Failure detection based on heartbeat timeout
- Epoch-based stale request rejection
- Role reassignment during reconfiguration
- ACK propagation from tail to head
- Transit-log compaction after acknowledgements

## Testing

The file `Test Checklist.md` contains a detailed validation plan covering:

- Functional tests
- Consistency tests
- Failure handling
- Recovery scenarios
- Stress testing
- Performance measurement
- Cleanup validation

This checklist is useful as the primary guide for verifying correctness and resilience of the system.

## Current Scope

This repository is a prototype implementation focused on system behavior and architecture rather than production hardening.

It already demonstrates:

- Multi-process distributed coordination
- Persistent metadata management
- Replicated chunk writes
- Streaming chunk retrieval
- Reconfiguration logic for node membership changes

## Possible Future Improvements

Natural next steps for this project could include:

- A clearer end-user CLI for upload, retrieve, and delete
- Better bootstrap and setup documentation
- Automated integration tests
- Stronger recovery behavior for partially completed operations
- Multi-chain chunk placement policies
- Checksums and corruption detection
- Authentication and authorization
- Metrics, observability, and tracing

## Summary

This project is a distributed object storage prototype built in Go that combines a router-based metadata plane with a master-managed chain-replicated storage plane. Files are chunked, replicated through storage nodes, and reconstructed from tail reads, while epochs and heartbeats help the cluster respond to failures and topology changes.
