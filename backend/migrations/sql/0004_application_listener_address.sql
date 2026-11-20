ALTER TABLE applications
    ADD COLUMN listening_address text NOT NULL DEFAULT '',
    ADD COLUMN listening_address_pending boolean NOT NULL DEFAULT false;
