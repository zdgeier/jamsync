echo "Starting building..."

mkdir -p ./jamsync-build/public

ARRAY=( "linux:amd64"
        "linux:arm64"
        "darwin:amd64"
        "darwin:arm64" )

for kv in "${ARRAY[@]}" ; do
    KEY=${kv%%:*}
    VALUE=${kv#*:}
    env GOOS=$KEY GOARCH=$VALUE go build -o ./jamsync-build/public/jam -ldflags "-s -w" cmd/client/main.go 

    if [[ "$KEY" == "darwin" ]]
    then
        codesign -s $CODESIGN_IDENTITY -o runtime -v ./jamsync-build/public/jam
    fi

    zip -m ./jamsync-build/public/jam_${KEY}_${VALUE}.zip ./jamsync-build/public/jam

    if [[ "$KEY" == "darwin" ]]
    then
        xcrun notarytool submit ./jamsync-build/public/jam_${KEY}_${VALUE}.zip --keychain-profile "AC_PASSWORD" --wait
    fi
done

echo "Done building!"
