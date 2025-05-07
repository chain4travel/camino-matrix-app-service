CREATE TABLE chunked_messages (
    message_id                VARCHAR(150)  NOT NULL PRIMARY KEY,
    stored_chunks_number      BIGINT        NOT NULL,
    expected_chunks_number    BIGINT        NOT NULL
);