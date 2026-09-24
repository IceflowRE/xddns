# Changelog

## 0.2.1

### Updated

- Performance improvements

### Changed

- Only notify the most recent error (the full error chain is still logged)
- `public_prefix` in IONOS provider is now a sensitive string
- Decreased timeout of ipservice resolver

## 0.2.0

### Added

- Support for `shell` resolver driver, which executes a shell command
- Update systemd service file to restart only once again after a failure
- Fix systemd service permissions

## 0.1.0

- Initial release
