-- Schema for the Montana Route Planner (PostgreSQL + PostGIS).
-- Files in db/ run in name order the first time the database container starts.

-- Mounting db/ over the init folder replaces the image's own PostGIS setup
-- script, so the extension gets created here.
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE highways (
  id   SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  geom geometry(LineString, 4326) NOT NULL
);

CREATE TABLE border (
  id   SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  geom geometry(Polygon, 4326) NOT NULL
);

CREATE TABLE pois (
  id   SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('city', 'pass', 'sasquatch')),
  geom geometry(Point, 4326) NOT NULL
);

CREATE INDEX highways_geom_idx ON highways USING GIST (geom);
CREATE INDEX pois_geom_idx ON pois USING GIST (geom);

CREATE TABLE users (
  id            SERIAL PRIMARY KEY,
  username      VARCHAR(50) NOT NULL UNIQUE,
  password_hash TEXT NOT NULL
);

-- Every planned route gets logged here, logged in or not
CREATE TABLE search_log (
  id             BIGSERIAL PRIMARY KEY,
  start_geom     geometry(Point, 4326) NOT NULL,
  end_geom       geometry(Point, 4326) NOT NULL,
  start_highway  TEXT NOT NULL,
  end_highway    TEXT NOT NULL,
  highways       TEXT[] NOT NULL,
  distance_miles NUMERIC(7, 1) NOT NULL,
  checkpoints    INT NOT NULL,
  searched_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Admin account, password is admin123 (bcrypt hash)
INSERT INTO users (username, password_hash)
VALUES ('admin', '$2y$10$yJOCmA8egUlzDV4Rp3QH.eEMv.raovBIJwWyP2Kwu432CgdVcM0Ra');
