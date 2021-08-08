`kopia` shallow restores of `git-annex` trees don't
work well: in particular there's tediousness about
restoring the actual object tree files corresponding
to the `git-annex` symlinks. This is a helper to
fix this up.
