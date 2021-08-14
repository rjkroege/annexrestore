`kopia` shallow restores of `git-annex` trees don't work well: in
particular there's tediousness about restoring the actual object tree
files corresponding to the `git-annex` symlinks. This is a helper to
fix this up: it does a deep restore of each symlink that points into
the `git-annex` object tree and replaces the symlink with the actual
file. Re-snapshotting the resultant (probably mostly shallow) tree
will then include the actual file instead of a symlink into (probably)
nowhere.

