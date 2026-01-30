IndexedDB Design (Web Frontend MVP)
- Stores:
  - tasks
    * keyPath: local_id
    * fields: local_id, server_id, user_id, server_version, title, description, status, priority, due_at, created_at, updated_at, completed_at, is_deleted, last_modified
    * indexes: by_user_id, by_status, by_due_at, by_last_modified
  - delta_queue
    * keyPath: id
    * fields: id, local_id, op, payload, client_version, timestamp
  - sync_meta
    * keyPath: user_id
    * fields: last_sync_at, last_server_version
  - conflicts
    * keyPath: id
    * fields: id, local_id, server_id, reason, options, created_at
- API surface (web):
  - addTask, updateTask, deleteTask
  - enqueueDelta(change), flushDeltaToSync()
  - exportData(format) and importData(file)
- Offline behavior:
  - Use a service worker for cache and background sync prompts.
- Notes:
  - Local data should be constrained to the current user; careful about cross-user data leakage.
