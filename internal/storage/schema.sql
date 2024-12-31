-- Session metadata (one record per device in a multi-device capture)
CREATE TABLE session_info (
    id INTEGER PRIMARY KEY,
    start_time DATETIME NOT NULL,
    device_type TEXT NOT NULL,    -- 'rtl-sdr' or 'hackrf'
    device_id TEXT NOT NULL,      -- Serial number or unique identifier
    config_json TEXT NOT NULL,    -- Device config
    UNIQUE(device_id, start_time) -- Prevent duplicate device sessions
);

-- Telemetry data
CREATE TABLE telemetry (
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
    radio_rssi INTEGER,          -- Radio link RSSI
    FOREIGN KEY(session_id) REFERENCES session_info(id) ON DELETE CASCADE
);

-- Core samples table
CREATE TABLE samples (
    id INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL,  -- Link back to capturing session
    timestamp DATETIME NOT NULL,  -- Time of the measurement
    frequency REAL NOT NULL,      -- Center frequency in Hz
    bin_width REAL NOT NULL,      -- Frequency bin width in Hz
    power REAL,                   -- Signal power in dBm
    num_samples INTEGER NOT NULL, -- Number of samples in bin (NULL for HackRF)
    telemetry_id INTEGER,         -- Foreign key to telemetry data
    FOREIGN KEY(session_id) REFERENCES session_info(id) ON DELETE CASCADE,
    FOREIGN KEY(telemetry_id) REFERENCES telemetry(id) ON DELETE SET NULL
);

-- Essential indexes for joins and basic querying
CREATE INDEX idx_samples_session ON samples(session_id);
CREATE INDEX idx_samples_freq ON samples(frequency, timestamp);
CREATE INDEX idx_telemetry_session ON telemetry(session_id);
