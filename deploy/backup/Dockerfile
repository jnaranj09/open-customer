# Backup image (design doc item 84): the pinned Postgres image, so pg_dump and pg_restore match the
# server's major version, plus bash, restic and jq. Busybox already has flock (with -n), so no
# extra package is needed (checked at P7). The pin, digest included, is PG_IMAGE in the Makefile;
# make check-pins fails if this copy differs.
#
# The three packages come from the Alpine mirror unpinned; the base pins the Alpine release.
# Threat model T91 accepts bash and jq. restic is open (TASKS.md "Found, not fixed"): its official
# images fail make image-scan, so it is not copied from one yet (guard TestSupplyChainPins allows
# only these three).
FROM postgres:18.6-alpine@sha256:123b17998ccaf47e53b502e8c6a1e79afd8f579c6f3eec83c563da026f957395

RUN apk add --no-cache bash restic jq

# The base image's gosu is only used by its docker-entrypoint.sh, which this image never runs
# (ENTRYPOINT is backup.sh); it carries Go stdlib CVEs the image scan blocks on.
RUN rm /usr/local/bin/gosu

# The media volume's mount folder, owned by the app user (design doc "Volume ownership"). Alpine
# ships /media/cdrom, floppy and usb; Docker would copy them into an empty media volume, so the
# folder is made again, empty.
RUN rm -rf /media && mkdir /media && chown 65532:65532 /media

COPY backup.sh /backup.sh
RUN chmod 0755 /backup.sh

ENTRYPOINT ["/backup.sh"]
# The base image's CMD is ["postgres"]; without this reset it would become backup.sh's mode.
CMD []
