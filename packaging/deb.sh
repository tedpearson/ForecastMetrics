#!/usr/bin/env bash

dir=$(dirname "${BASH_SOURCE[0]}")

BINARY=$1; shift
VERSION=$1; shift
DEB_FILE=$1; shift

mkdir -p deb/usr/local/bin
cp $BINARY deb/usr/local/bin/
mkdir -p deb/usr/local/share/forecastmetrics
cp "$dir/../config/forecastmetrics.example.yaml" deb/usr/local/share/forecastmetrics/
mkdir -p deb/DEBIAN
cat > deb/DEBIAN/control <<-END
	Package: forecastmetrics
	Version: $VERSION
	Architecture: armhf
	Maintainer: Ted Pearson <ted@tedpearson.com>
	Description: Ingests forecast data into time-series database.
END

dpkg-deb --build --root-owner-group deb "$DEB_FILE"