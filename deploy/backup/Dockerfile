# Backup image (design doc item 84): the pinned Postgres image, so pg_dump and pg_restore match the
# server's major version, plus bash, restic and jq. Busybox already has flock (with -n), so no
# extra package is needed (checked at P7).
FROM postgres:18.6-alpine

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
