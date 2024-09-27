import { createDockerDesktopClient } from '@docker/extension-api-client';
import { ChangeEvent, useState } from 'react';
import './InstallView.css';

const ddClient = createDockerDesktopClient();

type InstallViewProps = {
    install: (installLocation: string | undefined) => void;
}

export default function InstallView({ install }: InstallViewProps) {
    const [usePath, setUsePath] = useState(false);
    const [path, setPath] = useState('');

    function onCheckboxStateChange(event: ChangeEvent<HTMLInputElement>) {
        setUsePath(event.target.checked);
    }

    async function browse() {
        try {
            const { filePaths } = await ddClient.desktopUI.dialog.showOpenDialog({ properties: ['openDirectory', 'createDirectory', 'promptToCreate', 'dontAddToRecent'] });
            const selectedPath = filePaths.shift();
            if (selectedPath) {
                setPath(selectedPath);
            }
        } catch (ex) {
            console.error(ex);
            setUsePath(false);
        }
    }

    function isInputsValid() {
        // Either we don't use path, or path is not empty.
        return !usePath || !!path;
    }

    function triggerInstall() {
        install(usePath ? path : undefined);
    }

    return <div className="install-wrapper">
        <h2>Ollama needs to be installed</h2>
        <div className="options">
            <label>
                <input type="checkbox" checked={usePath} onChange={onCheckboxStateChange} />
                Install Ollama to custom location
            </label>
            <input type="text" readOnly value={path} disabled={!usePath} />
            <button onClick={browse} disabled={!usePath}>Browse...</button>
        </div>
        <button onClick={triggerInstall} disabled={!isInputsValid}>Install</button>
    </div>;
}
