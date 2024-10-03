import './InstallView.css';

type InstallViewProps = {
    install: () => void;
}

export default function InstallView({ install }: InstallViewProps) {
    return <div className="install-wrapper">
        <h2>Ollama needs to be installed</h2>
        <button onClick={install}>Install</button>
    </div>;
}
