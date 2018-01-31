VST2_VERSION="0.1.1"

#get go params
GOPATH="$(go env GOPATH)" 

#get vst2 dependency
curl -L -s https://github.com/dudk/vst2/releases/download/v${VST2_VERSION}/vst2-darwin-amd64.zip -o vst2_darwin_amd64.zip 

#install vst2 dependency
unzip vst2_darwin_amd64.zip -d $GOPATH/ 

#clean up
rm vst2_darwin_amd64.zip