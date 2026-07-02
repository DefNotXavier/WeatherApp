-- Create the database
CREATE DATABASE IF NOT EXISTS montana_planner;
USE montana_planner;



-- Create and grant user
CREATE USER IF NOT EXISTS 'montanauser'@'localhost' IDENTIFIED BY 'MontanaPass$';
GRANT ALL PRIVILEGES ON montana_planner.* TO 'montanauser'@'localhost';
FLUSH PRIVILEGES;

START TRANSACTION;

-- Users table
CREATE TABLE IF NOT EXISTS users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(50) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL
)ENGINE=InnoDB;

-- Highway points table
CREATE TABLE IF NOT EXISTS points (
  id INT AUTO_INCREMENT PRIMARY KEY,
  highway VARCHAR(10),
  name VARCHAR(100),
  lat DECIMAL(9,6),
  lng DECIMAL(9,6),
  sequence_order INT,
  point_type VARCHAR(20) ,
  nearest_highway_pt VARCHAR(6)
)ENGINE=InnoDB;

-- Search log table
CREATE TABLE IF NOT EXISTS search_log (
  id INT AUTO_INCREMENT PRIMARY KEY,
  start_lat DECIMAL(9,6),
  start_lng DECIMAL(9,6),
  start_highway VARCHAR(10),
  end_lat DECIMAL(9,6),
  end_lng DECIMAL(9,6),
  end_highway VARCHAR(10),
  route_length INT,
  searched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Insert admin user with password has for admin123
-- May look strange but I manually hased admin123 using the hash function, and copy and pasted here
INSERT INTO users (username, password_hash) 
VALUES ('admin', '$2y$10$yJOCmA8egUlzDV4Rp3QH.eEMv.raovBIJwWyP2Kwu432CgdVcM0Ra');

COMMIT;