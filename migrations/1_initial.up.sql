CREATE TABLE cheques (
    chequebook_id    VARCHAR(150)   NOT NULL PRIMARY KEY,
    from_cm_account  VARCHAR(50)    NOT NULL,
    to_cm_account    VARCHAR(50)    NOT NULL,
    to_bot           VARCHAR(50)    NOT NULL,
    counter          BIGINT         NOT NULL,
    amount           BIGINT         NOT NULL,
    created_at       BIGINT         NOT NULL,
    expires_at       BIGINT         NOT NULL,
    signature        VARBINARY(128) NOT NULL,
    tx_id            VARCHAR(50),
    status           TINYINT
);

CREATE TABLE chunked_messages (
    message_id                VARCHAR(150)  NOT NULL PRIMARY KEY,
    stored_chunks_number      BIGINT        NOT NULL,
    expected_chunks_number    BIGINT        NOT NULL
);

CREATE TABLE jobs (
    name        VARCHAR(150)  NOT NULL PRIMARY KEY,
    execute_at  BIGINT        NOT NULL,
    period      BIGINT        NOT NULL
);

