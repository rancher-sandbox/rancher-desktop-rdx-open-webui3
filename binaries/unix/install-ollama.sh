#!/bin/bash
CHECK_URL="http://localhost:11434/api/tags"
APP_NAME="ollama"

function install_ollama_on_linux {
    APP_INSTALLATION_PATH="./$APP_NAME"
    
    echo "Checking if $CHECK_URL returns a valid response..."
    if curl --output /dev/null --silent --head --fail "$CHECK_URL"; then
        echo "Ollama seems to be running correctly."
    else
        echo "The API check failed, checking if $APP_NAME is available..."
        if [ -f "$APP_INSTALLATION_PATH" ]; then
            echo "Ollama installation $APP_INSTALLATION_PATH found on the machine, let's run it"
            "$APP_INSTALLATION_PATH" serve &
            sleep 5
            "$APP_INSTALLATION_PATH" pull tinyllama
        else
            echo "Ollama installation not found on the machine, let's install it"
            curl -L https://ollama.com/download/ollama-linux-amd64 -o "$APP_INSTALLATION_PATH"
            chmod +x "$APP_INSTALLATION_PATH"
            "$APP_INSTALLATION_PATH" serve &
            sleep 5
            "$APP_INSTALLATION_PATH" pull tinyllama
        fi
    fi
}

function install_ollama_on_macos {
    DOWNLOAD_URL=https://ollama.com/download/Ollama-darwin.zip
    ZIP_FILE="Ollama-darwin.zip"
    APP_INSTALLATION_PATH="/Applications/Ollama.app"
    
    echo "Checking if $CHECK_URL returns a valid response..."
    if curl --output /dev/null --silent --head --fail "$CHECK_URL"; then
        echo "Ollama seems to be running correctly."
    else
        echo "The API check failed, checking if $APP_NAME is available..."
        if [ -d "$APP_INSTALLATION_PATH" ]; then
            echo "$APP_INSTALLATION_PATH found on the machine, let's run it"
            open -a "$APP_INSTALLATION_PATH"
            sleep 5
            "$APP_INSTALLATION_PATH/Contents/MacOS/$APP_NAME" pull tinyllama
        else
            echo "$APP_INSTALLATION_PATH not found on the machine, let's install it"
            curl -L "$DOWNLOAD_URL" -o "$ZIP_FILE"
            unzip "$ZIP_FILE" -d /Applications/
            open -a "$APP_INSTALLATION_PATH"
            sleep 5
            "$APP_INSTALLATION_PATH/Contents/MacOS/$APP_NAME" pull tinyllama
        fi
    fi
}

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    install_ollama_on_linux
elif [[ "$OSTYPE" == "darwin"* ]]; then
    install_ollama_on_macos
else

echo "Done."
