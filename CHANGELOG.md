# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Initial release: extracted from `oddbit-project/blueprint` `sqlb`.

### Changed

- Root package renamed `sqlb` → `gohan`; error strings' prefix changed from `"sqlb: "` to
  `"gohan: "`.
- Struct-metadata helpers moved from Blueprint's `db/field` into this module's own `field`
  subpackage, with no dependency on Blueprint.
