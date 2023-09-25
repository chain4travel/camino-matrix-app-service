CREATE TABLE cheques (
    chequebook_id VARCHAR(150)   NOT NULL PRIMARY KEY,
    issuer        VARCHAR(50)    NOT NULL,
    agent         VARCHAR(50)    NOT NULL,
    beneficiary   VARCHAR(50)    NOT NULL,
    amount        BIGINT         NOT NULL,
    serial_id     BIGINT         NOT NULL,
    signature     VARBINARY(128) NOT NULL,
    tx_id         VARCHAR(50),
    status        TINYINT
);