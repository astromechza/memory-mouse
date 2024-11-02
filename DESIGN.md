# A - first thoughts

In general there are 3 main routes:

- POST /documents - create an empty blank document and save it in object storage
- GET /documents/<ID> - get the document
- PUT /documents/<ID> - stream changes to and from the doc
- DELETE /documents/<ID> - delete the remote doc from object storage

We want to minimise state here. So there's no database of documents, instead we query object storage to determine if it exists or not and to operate on it - because docs are cached in memory we automatically have a cache here for better performance.

We definitely want to also support listing documents for the current user. A `GET /documents` route. This requires having some metadata next to each document indicating the ownership.

Most object storage APIs support tagging or metadata with documents, which is good, and can be modified on objects when reassigned - however it would be good to also support a file system backed storage API for demos and local testing, and file systems don't really support "tagging" in the same way.

So instead we're going to use sqlite as the backing file system so that the rows can be written as a name + BLOB + metadata. Sqlite is generally better than DuckDB for this usecase with few readers and low row aggregations. We must just make sure to keep the incremental save points smaller than 1GB or so. Which should be ok. Then onobject storage we use the x-amzn-meta-* headers.

# B - second thoughts

listing documents is a lot easier when we have a known user or project id to list them under as ownership. I think we should introduce a project id which holds a list of docs for either the user or a group of users. It's up to the authz / session middleware to return a header or claim indicating which project the current request is under.

So this keeps our routes the same, but a middleware can optionally prefix all paths with /<project id>/ or do everything by a header.

So let's run through what CreateDocument does:

1. Validate that the current user is able to make a request against the given project id. This is AuthZ but also serves to ensure the project exists.
2. Generate a new document id.
3. Generate an empty automerge document and save it to []byte
4. Make a PutObject request with the []byte, sha256 checksum, to <bucket>/project/<project>/documents/<document>/00000000001
5. Load the document into memory

We then start a goroutine for each document in memory:

1. Every N seconds, check how many total changes have been made, if above B bytes, increment the chunk number and add it to our outgoing chunk queue or if M seconds have elapsed since the last chunk, also increment and add it to the outgoing list.
2. Ensure this is locked sufficiently, so that the document doesn't get flushed from ram, and the server doesn't stop while it's changes are still being flushed. If it does, ensure things are fenced off correctly so that another copy of the server can correctly load up.
3. In the outgoing chunk queue, write each chunk, in order, via PutObject.
4. If no connections to the doc have been present for more than C seconds, flush the chunk queue, and drop the document from memory.

This leads us to the process for preparing for a PutDocument call.

1. If document is loaded already, attempt to open a sync session (this should work if fully loaded).
2. Otherwise, list the objects under project and document, and begin loading them in order.
3. Once fully loaded, allow sync sessions.

And this points to how we can collapse objects together.

1. Read chunk objects N, N+1, ..., N+n. generate the changes for the same range. Overwrite object N+n with the same changes.
2. Delete objects N.., N+n-1 in reverse order.
3. This compacts the files and ensures that any concurrent loading will always end up loaded the set of changes, potentially twice.


