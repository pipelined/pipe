GOPATH="$(go env GOPATH)" 
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

cd "${GOPATH}/src/github.com/dudk/private.vst2/_vst2"

./build.sh

ARCHIVE="vst2-${GOOS}-${GOARCH}.zip"

cp "${ARCHIVE}" "${GOPATH}/src/github.com/dudk/phono"

TARGET_PATH="${GOPATH}/pkg/${GOOS}_${GOARCH}/github.com/dudk/phono/vendor/github.com/dudk"

#install vst2 dependency binary
mkdir -p "${TARGET_PATH}"
unzip -joq ${ARCHIVE} *.a -d "${TARGET_PATH}"

#clean up
rm "${ARCHIVE}"
rm "${GOPATH}/src/github.com/dudk/phono/${ARCHIVE}"