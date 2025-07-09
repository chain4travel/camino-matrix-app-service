CREATE TABLE chunked_messages (
    message_id               VARCHAR(150)  NOT NULL PRIMARY KEY,
    stored_chunks_count      INTEGER       NOT NULL,
    expected_chunks_count    INTEGER       NOT NULL
);