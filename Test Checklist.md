# Distributed Object Storage Testing Checklist

## Phase 1 — Functional Tests

### Upload

* [ ] Upload a small file (< 1 chunk)
* [ ] Upload exactly one chunk (64 MB)
* [ ] Upload a multi-chunk file
* [ ] Upload a large file (1 GB+)
* [ ] Upload two files having the same name
* [ ] Upload multiple files sequentially
* [ ] Upload multiple files concurrently

### Retrieval

* [ ] Retrieve a small file
* [ ] Retrieve a large file
* [ ] Compare SHA-256 hash of original vs retrieved file
* [ ] Retrieve immediately after upload
* [ ] Retrieve the same file multiple times
* [ ] Retrieve multiple files concurrently
* [ ] Retrieve a non-existent object
* [ ] Retrieve after deletion

### Delete

* [ ] Delete an existing object
* [ ] Verify all nodes deleted their object directory
* [ ] Verify router metadata is removed
* [ ] Delete the same object twice (idempotency)
* [ ] Delete a non-existent object
* [ ] Delete immediately after upload
* [ ] Delete after retrieval

---

# Phase 2 — Consistency Tests

## Replication

* [ ] Verify every chunk exists on every node of the chain
* [ ] Verify sequence numbers are identical across replicas
* [ ] Verify acknowledgements reach the head
* [ ] Verify metadata is only committed after successful upload

## Object Reconstruction

* [ ] Verify chunk ordering
* [ ] Verify last chunk size
* [ ] Verify completeness check rejects missing chunks
* [ ] Verify temporary dump directory is cleaned

---

# Phase 3 — Failure Handling

## Upload Failures

### Head Crashes

* [ ] Before local write
* [ ] After local write
* [ ] Before forwarding
* [ ] After forwarding
* [ ] After ACK

### Middle Crashes

* [ ] Before write
* [ ] After write
* [ ] Before forwarding
* [ ] After forwarding

### Tail Crashes

* [ ] Before write
* [ ] After write
* [ ] Before ACK
* [ ] After ACK

### Network Failures

* [ ] Successor unreachable
* [ ] ACK lost
* [ ] Client retries upload
* [ ] Duplicate upload request

---

## Retrieval Failures

* [ ] Tail unavailable
* [ ] One chain unavailable
* [ ] Missing chunk
* [ ] Corrupted chunk
* [ ] Client disconnects during retrieval

---

## Delete Failures

* [ ] Head crashes before forwarding delete
* [ ] Middle crashes
* [ ] Tail crashes before ACK
* [ ] Delete request replay
* [ ] Delete after object already removed

---

## Epoch Tests

* [ ] Reject stale epoch write
* [ ] Reject stale delete
* [ ] Reject stale ACK
* [ ] Accept current epoch
* [ ] Reconfigure cluster
* [ ] Verify old configuration cannot write

---

## Recovery Tests

* [ ] Restart node
* [ ] Restart master
* [ ] Restart router
* [ ] Restart entire cluster
* [ ] Verify data remains readable

---

## Stress Tests

* [ ] 100 uploads
* [ ] Concurrent uploads
* [ ] Concurrent retrievals
* [ ] Upload while retrieving
* [ ] Upload while deleting
* [ ] Delete while retrieving
* [ ] Large file (5 GB+ if disk permits)

---

## Performance Measurements

* [ ] Measure upload throughput (MB/s)
* [ ] Measure retrieval throughput (MB/s)
* [ ] Measure delete latency
* [ ] Measure metadata lookup latency
* [ ] Measure memory usage during retrieval
* [ ] Measure CPU utilization
* [ ] Measure disk usage before and after deletion

---

## Cleanup Tests

* [ ] Temporary directories removed
* [ ] No orphan metadata
* [ ] No orphan chunk directories
* [ ] No stale logs

---

# Notes

* Record bugs encountered during testing.
* Document root cause and resolution for every failure.
* Repeat the failing test after applying a fix.
* Keep a changelog of protocol or implementation changes discovered during testing.
