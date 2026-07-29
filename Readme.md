# Distributed Object Storage System

This project is my attempt at building a simple distributed object storage system in Go. The main goal was to understand how distributed storage can be designed when metadata management, replication, failure handling, and client interaction are all treated as separate concerns.

Instead of storing a file as one large blob, the system breaks it into chunks, sends those chunks through a storage chain, and keeps track of the object metadata through a router service backed by PostgreSQL.

## What I Built

The system is divided into three main parts:

- `client`
- `router`
- `storage`

Each part has a specific responsibility.

### Client

The client is used to trigger file operations:

- upload
- retrieve
- delete

For an upload, the client first contacts the router, gets an object ID and the active chain information, fetches the current epoch from the master, and then sends chunks to the head node.

For retrieval, the client asks the router for metadata, contacts the tail nodes to stream chunks back, and then reconstructs the file locally.

For deletion, the client again starts with the router, sends delete commands through the chain, and finally notifies the router when cleanup is complete.

### Router

The router acts as the metadata layer and the main public entrypoint for the system. It currently runs on `localhost:8000`.

Its responsibilities include:

- registering active chains
- creating metadata during upload initialization
- marking uploads as complete
- returning metadata required for retrieval
- returning metadata required for delete
- removing metadata once delete completes

Metadata is stored in PostgreSQL using GORM.

Each stored object has:

- `ID`
- `Key`
- `Size`
- `ChunkSize`
- `Status`
- timestamps

The current object lifecycle uses these states:

- `UPLOADING`
- `READY`
- `DELETED`

### Storage Subsystem

The storage side is made up of:

- one master
- multiple storage nodes

The master manages node membership and chain layout, while the nodes actually store and replicate the chunks.

## Core Idea

The design of this project is based mainly on chain replication.

When a write comes in:

1. it enters through the head node
2. it gets forwarded along the chain
3. the tail node finishes the write
4. an acknowledgement travels back upstream

Reads are served from the tail. The idea is that the tail reflects the latest committed state.

Deletes use the same chain path, except the command sent is `DELETE` instead of a normal chunk write.

## Master Responsibilities

The master is responsible for:

- registering nodes
- maintaining the active chain layout
- assigning node roles
- tracking the current epoch
- checking heartbeats
- detecting failed nodes
- reconfiguring the chain when membership changes
- registering the active chain with the router

The roles currently used in the chain are:

- `HEAD`
- `MIDDLE`
- `TAIL`
- `ORPHAN`

## Storage Node Responsibilities

Each storage node:

- loads its config from `node.json`
- registers itself with the master
- accepts reconfiguration commands
- stores chunks on local disk
- forwards writes to the next node
- sends acknowledgements upstream
- serves chunk reads from local storage

Each node also maintains a `log.txt` file that acts as a transit log for writes that have been forwarded but not yet acknowledged.

## Request Flow

### Upload

1. client sends upload initialization request to router
2. router creates metadata with `UPLOADING` status
3. router returns object ID and chain information
4. client fetches the current epoch
5. client splits the file into 64 MB chunks
6. client sends chunks to chain heads
7. nodes replicate the chunks through the chain
8. tail sends ACKs back upstream
9. client notifies router after upload completes
10. router marks the object as `READY`

### Retrieval

1. client asks router for object metadata
2. router returns object ID, topology, and chunk count
3. client requests chunk streams from tail nodes
4. client stores chunks temporarily
5. chunks are sorted by chunk ID
6. file is reconstructed locally

### Delete

1. client asks router to initialize delete
2. router returns object ID and chain info
3. client fetches the epoch
4. client sends `DELETE` requests to the head nodes
5. nodes delete their local object directories and forward the delete
6. client notifies router when delete is complete
7. router removes the metadata

## Project Structure

```text
Distributed Object Storage System/
|-- client/
|   |-- cmd/
|   |-- internal/
|   `-- go.mod
|-- router/
|   |-- cmd/
|   |-- internal/
|   |   |-- database/
|   |   |-- handlers/
|   |   |-- metadata/
|   |   `-- repositories/
|   `-- go.mod
|-- storage/
|   |-- cmd/
|   |   |-- master/
|   |   `-- node/
|   |-- internal/
|   |   |-- master/
|   |   |-- node/
|   |   `-- shared/
|   |-- master1/
|   |-- node1/
|   |-- node2/
|   |-- node3/
|   |-- cluster_start.ps1
|   |-- cluster_start.sh
|   `-- go.mod
|-- Architecture.png
|-- Readme.md
`-- Test Checklist.md
```

## Configuration

### Router Database Variables

The router expects these environment variables:

- `DB_HOST`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `DB_PORT`

### Master Configuration

The master reads its configuration using:

- `MASTER_PATH`

In the provided PowerShell setup, this points to:

- `./master1`

### Node Configuration

Each node reads its configuration using:

- `NODE_PATH`

The PowerShell helper scripts use:

- `./node1`
- `./node2`
- `./node3`

Each node config includes:

- node address
- master address
- node ID

## How to Run

### Start the Router

After setting up the PostgreSQL environment variables:

```powershell
cd router
go run ./cmd
```

### Start the Storage Cluster on Windows

```powershell
cd storage
.\cluster_start.ps1
```

This launches:

- one master
- three storage nodes

### Start the Storage Cluster on Linux

```bash
cd storage
./cluster_start.sh
```

### Run the Client

```powershell
cd client
go run .
```

The client is mainly set up for manual testing right now.

## Local Storage Layout

On disk, each node stores chunks in directories named after the object ID. Each chunk is stored as a file using its chunk number.

Conceptually it looks like this:

```text
node1/
`-- <object-id>/
    |-- 0
    |-- 1
    `-- 2
```

## Reliability Features Included

This project currently includes:

- heartbeats from nodes to master
- failure detection based on heartbeat timeout
- epoch-based stale request rejection
- role reassignment during reconfiguration
- acknowledgement propagation from tail to head
- log compaction after ACKs

## Testing

I also kept a `Test Checklist.md` file to organize the validation work. It includes:

- functional tests
- consistency tests
- failure handling cases
- recovery scenarios
- stress tests
- performance measurements
- cleanup checks

## Current Scope

This is a prototype project. The focus was more on understanding the distributed systems side of the problem than on making it production-ready.

What it already demonstrates:

- chunk-based object storage
- centralized metadata handling
- chain replication
- client-driven upload, retrieval, and delete flows
- node reconfiguration with epochs
- multi-process coordination

## Possible Improvements

If I continue working on this, some obvious next steps would be:

- improving the CLI experience
- adding stronger recovery for partial failures
- writing proper integration tests
- supporting better placement across multiple chains
- adding checksums for corruption detection
- adding metrics and observability
- improving setup and deployment documentation

## Final Note

This project was mainly built as a systems exercise to understand how storage nodes, replication, metadata, and failure handling fit together in a distributed object store. It is not production-grade, but it helped me explore the architecture and tradeoffs involved in building one from scratch.
