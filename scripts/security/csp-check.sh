#!/usr/bin/env bash
# The script CSP's sensor (docs/designs/devsecops.md section 6; threat model T28 and T29). The
# header CSP from the Go server has no script-src: the script policy is the <meta> tag SvelteKit
# writes from kit.csp in web/vite.config.ts. make web runs this on the real web/build/index.html
# right after `pnpm build` (the SPA has one page; adapter-static's fallback is that index.html).
#
# Red (exit 1) unless the file has exactly one <meta http-equiv="content-security-policy"> and its
# policy has:
#   - default-src 'self' and nothing else (object-src falls back to it; an object-src, if set, is
#     'none');
#   - script-src with 'self' and only 'sha256-...' hashes besides ('unsafe-inline', 'unsafe-eval',
#     a host or any other source is red), and no script-src-elem or script-src-attr, which would
#     replace it;
#   - no frame-ancestors (a browser ignores it in a <meta>; the Go header sets it, test
#     TestSecurityHeadersOnEveryResponse);
# and every inline <script> in the file hashes to a listed sha256.
#
# The web-stub (make web-stub, a one-line page so go:embed compiles) is not the build: it has no
# meta CSP, so this script is red on it. That is on purpose: the check never reads a file it cannot
# tell is a SvelteKit build as passing. Exit 2: bad arguments or no perl.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: csp-check.sh <built index.html>" >&2
  exit 2
fi
page=$1
if ! command -v perl >/dev/null 2>&1; then
  echo "csp-check needs perl on this machine (install it, then run again)" >&2
  exit 2
fi
[ -f "$page" ] || {
  echo "csp-check: $page does not exist (run make web)" >&2
  exit 1
}

perl -MDigest::SHA=sha256_base64 -e '
  use strict;
  use warnings;
  my $page = $ARGV[0];
  open(my $fh, "<", $page) or die "csp-check: cannot read $page\n";
  local $/;
  my $html = <$fh>;
  close $fh;
  my @red;
  my @metas = $html =~ /<meta\b[^>]*http-equiv\s*=\s*"content-security-policy"[^>]*>/gi;
  if (@metas != 1) {
    print STDERR "csp-check: $page has " . scalar(@metas) . " CSP <meta> tags, want 1 (the web-stub has none: run make web)\n";
    exit 1;
  }
  my ($policy) = $metas[0] =~ /\bcontent\s*=\s*"([^"]*)"/i;
  defined $policy or do { print STDERR "csp-check: the CSP <meta> has no content\n"; exit 1 };
  my %dir;
  for my $d (split /;/, $policy) {
    my ($name, @src) = split " ", $d;
    next unless defined $name;
    $name = lc $name;
    push @red, "directive $name is set twice" if exists $dir{$name};
    $dir{$name} = [@src];
  }
  my $default = join " ", @{ $dir{"default-src"} // [] };
  push @red, "default-src is \"$default\", want \"'\''self'\''\"" if $default ne "'\''self'\''";
  if (exists $dir{"object-src"}) {
    my $o = join " ", @{ $dir{"object-src"} };
    push @red, "object-src is \"$o\", want unset or \"'\''none'\''\"" if $o ne "'\''none'\''";
  }
  push @red, "frame-ancestors is in the <meta> (browsers ignore it there; the Go header sets it)" if exists $dir{"frame-ancestors"};
  for my $d ("script-src-elem", "script-src-attr") {
    push @red, "$d is set: it would replace script-src" if exists $dir{$d};
  }
  my %hashes;
  if (!exists $dir{"script-src"}) {
    push @red, "no script-src";
  } else {
    my $self = 0;
    for my $s (@{ $dir{"script-src"} }) {
      if ($s eq "'\''self'\''") { $self = 1; next }
      if ($s =~ /^'\''sha256-([A-Za-z0-9+\/]{43}=)'\''$/) { $hashes{$1} = 1; next }
      push @red, "script-src allows $s (only '\''self'\'' and sha256 hashes)";
    }
    push @red, "script-src has no '\''self'\''" unless $self;
  }
  my $inline = 0;
  while ($html =~ /<script\b([^>]*)>(.*?)<\/script\s*>/gis) {
    my ($attrs, $body) = ($1, $2);
    next if $attrs =~ /\bsrc\s*=/i;
    $inline++;
    my $h = sha256_base64($body) . "=";
    push @red, "inline script $inline hashes to sha256-$h, which script-src does not list" unless $hashes{$h};
  }
  if (@red) {
    print STDERR "csp-check: $page: $_\n" for @red;
    exit 1;
  }
  print "csp-check: $page: ok (" . scalar(keys %hashes) . " hash(es), $inline inline script(s))\n";
' "$page"
