## [1.9.8] - 2026-06-27

### 🐛 Bug Fixes

- *(debian)* Render compact RAID LUKS partitions (#90)
## [1.9.7] - 2026-06-27

### 🐛 Bug Fixes

- *(debian)* Support RAID1 LUKS and TPM unlock (#88)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.9.7 (#89)
## [1.9.6] - 2026-06-25

### 🐛 Bug Fixes

- *(static)* Support user-provided static slc templates (#86)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.9.6 (#87)
## [1.9.5] - 2026-06-24

### 🐛 Bug Fixes

- *(debian)* Stage LUKS TPM reenroll with generated helpers (#84)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.9.5 (#85)
## [1.9.4] - 2026-06-24

### ⚙️ Miscellaneous Tasks

- *(installer)* Append structured late commands (#82)
- *(release)* V1.9.4 (#83)
## [1.9.3] - 2026-06-24

### ⚙️ Miscellaneous Tasks

- *(debian)* Support TPM unlock for regular LUKS (#80)
- *(release)* V1.9.3 (#81)
## [1.9.2] - 2026-06-23

### 🐛 Bug Fixes

- *(debian)* Support plain regular LUKS installs (#78)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.9.2 (#79)
## [1.9.1] - 2026-06-23

### 🐛 Bug Fixes

- *(server)* Bind HTTP server to all interfaces by default (#75)
- *(debian)* Format regular LUKS root mapper (#77)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.9.1 (#76)
## [1.9.0] - 2026-06-19

### 🚀 Features

- *(mappings)* Add LUKS storage encryption schema (#72)
- *(debian)* Support LUKS install rendering (#73)

### 📚 Documentation

- *(plans)* Complete LUKS install profiles (#74)

### ⚙️ Miscellaneous Tasks

- *(depot)* Move CI to depot (#69)
- *(depot)* Review CI to depot  (#71)
- *(release)* V1.9.0 (#70)
## [1.8.1] - 2026-06-19

### 🐛 Bug Fixes

- *(tftp)* Quiet expected option aborts (#67)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.8.1 (#68)
## [1.8.0] - 2026-06-19

### 🚀 Features

- *(preseed)* Support Debian storage modes (#65)

### 🐛 Bug Fixes

- *(preseed)* Set Debian LVM volume group (#63)

### 📚 Documentation

- *(storage)* Update storage guides and mappings (#66)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.8.0 (#64)
## [1.7.1] - 2026-06-19

### 🐛 Bug Fixes

- *(preseed)* Default Debian storage mode to regular (#60)
- *(preseed)* Harden Debian storage wipe behavior (#62)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.7.1 (#61)
## [1.7.0] - 2026-06-19

### 🚀 Features

- Add runtime persistence and nested config schema (#48)
- Persist runtime event history (#49)
- Persist waiting server state (#50)
- Persist boot config references (#51)
- Expose persistent runtime reference APIs (#52)
- *(cli)* Add explicit run command (#55)
- *(cli)* Add runtime inspection foundation (#56)
- *(cli)* Add event inspection commands (#57)
- *(cli)* Add server state inspection commands (#58)
- *(cli)* Add boot session inspection command (#59)

### 🐛 Bug Fixes

- *(handlers)* Render boot refs with stored environment (#54)

### 🚜 Refactor

- Make debug CLI-only (#46)

### 📚 Documentation

- Finalize runtime persistence operations (#53)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.7.0 (#47)
## [1.6.1] - 2026-06-18

### 🐛 Bug Fixes

- Make structured repo release canonical (#45)

### 🚜 Refactor

- Add slog-backed component logging (#42)
- Switch HTTP router to chi (#44)

### ⚙️ Miscellaneous Tasks

- *(release)* V1.6.1 (#43)
## [1.6.0] - 2026-06-18

### 🚀 Features

- *(mappings)* Add structured wipe disk selectors (#40)

### ⚙️ Miscellaneous Tasks

- *(go)* Update toolchain and docs (#39)
- *(release)* V1.6.0 (#41)
## [1.5.0] - 2026-06-18

### 🚀 Features

- *(cli)* Replace flag parsing with urfave cli (#32)
- Debian 13 (#34)
- Embed web UI assets into binary (#31)
- Add target-based provisioning mappings (#37)
- *(prov)* Embed structured provisioning defaults (#38)

### 🚜 Refactor

- *(config)* Load structured configs with koanf (#33)
- Move internal packages to root (#36)

### 🧪 Testing

- Migrate tests to testify and expand coverage (#30)

### ⚙️ Miscellaneous Tasks

- Change dry run to only run on release PRs (#28)
- *(release)* Restore semver release flow (#35)
- *(release)* V1.5.0 (#29)
## [1.4.7] - 2026-06-12

### ⚙️ Miscellaneous Tasks

- *(release)* Publish per-platform Shoelaces artifacts (#26)
- *(release)* V2026-06-12.05 (#27)
## [1.4.6] - 2026-06-12

### ⚙️ Miscellaneous Tasks

- *(release)* Split out the release and tag operations due to aws credentials (#24)
- *(release)* V2026-06-12.04 (#25)
## [1.4.5] - 2026-06-12

### ⚙️ Miscellaneous Tasks

- *(release)* Fix workflow permissions (#22)
- *(release)* V2026-06-12.03 (#23)
## [1.4.4] - 2026-06-12

### 💼 Other

- *(dep)* Bump golang.org/x/net from 0.23.0 to 0.38.0 (#2)

### ⚙️ Miscellaneous Tasks

- *(release)* Automate shoelaces release flow (#19)
- Consolidate shoelaces release publishing (#21)
- *(release)* V2026-06-12.02 (#20)
## [1.3.2] - 2023-04-19

### 🚀 Features

- Support custom parameters in integ_test

### 💼 Other

- How client and server interact during poll

### 🎨 Styling

- Add clarifying comments in polling iPXE scripts

### 🧪 Testing

- Add test for /start endpoint (#23)
## [1.3.1] - 2023-01-05

### 💼 Other

- Use go 1.19 and tidy up deps
## [1.3.0] - 2023-01-05

### 🚀 Features

- Add human-friendly entry point

### 💼 Other

- Update dependencies

### 🧪 Testing

- Ensures compatibility with Python 3
- Fix test for behavior modified in 3233a856
## [1.2.0] - 2021-01-13

### 💼 Other

- Add docker build
## [1.0.0] - 2018-08-03
