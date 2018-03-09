#TODO extract version from Gopkg.toml
VST2_VERSION="0.1.6"

#TODO add goos and goarch params
#get go params
GOPATH="$(go env GOPATH)" 
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

#archive name
ARCHIVE=vst2_${GOOS}_${GOARCH}.zip

#get vst2 dependency
curl -L -s https://github.com/dudk/vst2/releases/download/v${VST2_VERSION}/vst2-${GOOS}-${GOARCH}.zip -o ${ARCHIVE} 

TARGET_PATH="${GOPATH}/pkg/${GOOS}_${GOARCH}/github.com/dudk/phono/vendor/github.com/dudk"

#install vst2 dependency binary
mkdir -p "${TARGET_PATH}"
unzip -joq ${ARCHIVE} *.a -d "${TARGET_PATH}"

#clean up
rm ${ARCHIVE} 