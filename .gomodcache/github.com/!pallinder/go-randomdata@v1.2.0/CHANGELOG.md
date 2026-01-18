# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.0] - 2019-06-02
### Added
- Spaces in postal code for GB.
- Add functionality to randomly generate alphanumeric text.

### Fixed
- Fix race condition that was introduced by relying on privateRand.
- Fix title with random gender not actually generating a title

## [1.1.0] - 2018-10-31

### Added
- Generate random locale strings
- Country localised street names
- Country localised provinces

### Fixed
- Generating dates will respect varying number of days in a month

## [1.0.0] - 2018-10-30

### Added
- This CHANGELOG file to hopefully serve as an evolving example of a
  standardized open source project CHANGELOG.
- Enforcing Semver compatability in releases

### Changed
- Update README.md to include information about release strategy
- Update README.md to link to CHANGELOG.md
