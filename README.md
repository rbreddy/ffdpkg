### Firefox Dev Package Manager
This is to install/update the Firefox Dev edition.
There is no package for it (afaik).

There should be a better solution than this (like use Arch Linux).
At this moment it deletes the folder and will nuke user data. Be careful if you want to use this. Or just don't use it.

## TO-DO

1. Separate files: main.go, fetch.go, verify.go, install.go.
2. Cache dir: ensure userCacheDir() creates it and use consistently.
3. Atomic install: create temp dir as sibling of installDir, not inside it.
4. Profiles/data: do not store user data in /opt; use $HOME/.mozilla/firefox.
5. State file: ensure atomic write + fsync; copyStateAtomic() is okay.
6. Permissions: review 0700/0755 for temp dirs and final install.
7. Flags: install requires --cache; verify requires --tarball & --sig.
8. Error handling: return errors, avoid log.Fatal inside functions.
9. Extraction: handle tar.xz safely, preserve symlinks and permissions.
10. Optional improvements: symlink-based versioning for easy rollback.



