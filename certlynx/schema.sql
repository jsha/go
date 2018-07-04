CREATE TABLE names (
  reversedName TEXT,
  notAfter DATETIME,
  issuerID INT,
  certificateID INT
);

CREATE INDEX names_notAfter ON names (notAfter);
CREATE INDEX names_reversedName_notAfter ON names (reversedName, notAfter);

CREATE TABLE fqdnSets (
  fqdnSetSHA256 BLOB,
  notAfter DATETIME,
  issuerID INT,
  certificateID INT
);

CREATE INDEX fqdnSets_notAfter ON fqdnSets (notAfter);
CREATE INDEX fqdnSets_fqdnSetSHA256_notAfter ON fqdnSets (fqdnSetSHA256, notAfter);

CREATE TABLE certificates (
  sha256 BLOB UNIQUE PRIMARY KEY,
  serial TEXT,
  notAfter DATETIME,
  pre BOOL,

  issuerID INT
);

CREATE INDEX certificates_issuerID_notAfter ON certificates (issuerID, notAfter);

CREATE TABLE issuers (
  issuer TEXT UNIQUE PRIMARY KEY
);

CREATE TABLE logEntries (
  logID INT,
  logIndex INT,
  certificateID INT
);

CREATE INDEX logEntries_certificateID ON logEntries (certificateID);
CREATE INDEX logEntries_logID_logIndex ON logEntries (logID, logIndex);

CREATE TABLE logs (
  URL TEXT UNIQUE PRIMARY KEY
);

CREATE INDEX logs_URL ON logs (URL);

-- Example queries:
--
-- SELECT COUNT(DISTINCT(reversedName)) FROM certificates JOIN names ON issuerID = X AND certificates.ROWID = names.certificateID AND notAfter > datetime('now');
-- SELECT COUNT(DISTINCT(hash)) FROM certificates JOIN fqdnSets ON issuerID = X AND certificates.ROWID = fqdnSets.certificateID AND notAfter > datetime('now');
-- SELECT COUNT(DISTINCT(issuerID, serial)) FROM certificates where issuerID = X AND notAfter > datetime('now');
-- SELECT issuer, serial, notAfter, certificates.ROWID FROM certificates JOIN names ON certificates.ROWID = fqdnSets.certificateID ORDER BY notAfter DESC;
-- SELECT logID, logIndex FROM logEntries JOIN certificates ON certificates.hash = logEntries.hash AND certificates.ROWID = 1234;
-- SELECT MAX(logIndex) FROM logEntries JOIN logs WHERE URL = "https://example.com" AND logs.ROWID = logID;
