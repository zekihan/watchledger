-- Local action time is not portable authority. Imports/restores leave it NULL,
-- so imported source records cannot overturn a later app unwatch in another view.
ALTER TABLE history ADD COLUMN local_action_at timestamptz;
