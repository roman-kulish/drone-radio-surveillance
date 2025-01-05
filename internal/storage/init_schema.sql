-- Session metadata (one record per device in a multi-device capture)
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY,
    start_time DATETIME NOT NULL, -- session start time
    device_type TEXT NOT NULL,    -- 'rtl-sdr' or 'hackrf'
    device_id TEXT NOT NULL,      -- Serial number or unique identifier
    config TEXT NOT NULL,         -- Device config
    UNIQUE(device_id, start_time) -- Prevent duplicate device sessions
);

-- Core samples table
CREATE TABLE IF NOT EXISTS samples (
    id INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL,  -- Link back to capturing session
    timestamp DATETIME NOT NULL,  -- Time of the measurement
    frequency REAL NOT NULL,      -- Center frequency in Hz
    bin_width REAL NOT NULL,      -- Frequency bin width in Hz
    power REAL,                   -- Signal power in dBm
    num_samples INTEGER NOT NULL, -- Number of samples in bin (NULL for HackRF)
    telemetry_id INTEGER,         -- Foreign key to telemetry data
    FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY(telemetry_id) REFERENCES telemetry(id) ON DELETE SET NULL
);

-- Telemetry data
CREATE TABLE IF NOT EXISTS telemetry (
    id INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL, -- Link to capturing session
    timestamp DATETIME NOT NULL, -- Time of telemetry measurement
    latitude REAL,               -- GPS latitude
    longitude REAL,              -- GPS longitude
    altitude REAL,               -- Altitude in meters
    roll REAL,                   -- Roll angle in degrees
    pitch REAL,                  -- Pitch angle in degrees
    yaw REAL,                    -- Yaw angle in degrees
    accel_x REAL,                -- X-axis acceleration
    accel_y REAL,                -- Y-axis acceleration
    accel_z REAL,                -- Z-axis acceleration
    ground_speed REAL,           -- Ground speed in m/s
    ground_course REAL,          -- Ground course in degrees
    radio_rssi INTEGER,          -- Radio link RSSI
    FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE VIEW IF NOT EXISTS v_samples_with_telemetry AS
SELECT
    s.session_id,
    s.timestamp,
    s.frequency,
    s.bin_width,
    s.power,
    s.num_samples,
    s.telemetry_id,
    t.latitude,
    t.longitude,
    t.altitude,
    t.roll,
    t.pitch,
    t.yaw,
    t.accel_x,
    t.accel_y,
    t.accel_z,
    t.ground_speed,
    t.ground_course,
    t.radio_rssi
FROM samples s
LEFT JOIN telemetry t ON s.telemetry_id = t.id;

