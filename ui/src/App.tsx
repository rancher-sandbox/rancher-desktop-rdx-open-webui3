import { useState, useEffect } from 'react';
import { createDockerDesktopClient } from '@docker/extension-api-client';
import WebpageFrame from './WebpageFrame';
import InstallView from './InstallView';
import LoadingView from './LoadingView';

const ddClient = createDockerDesktopClient();

export function App() {
  const [error, setError] = useState('');
  const [checked, setChecked] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [installed, setInstalled] = useState(false);
  const [started, setStarted] = useState(false);
  const executable = `installer${ddClient.host.platform === 'win32' ? '.exe' : ''}`;

  async function runInstaller(...args: string[]) {
    const { host } = ddClient.extension;
    if (!host) {
      throw new Error(`Extension API does not have host`);
    }
    return await host.cli.exec(executable, args);
  }

  // On load, check if Ollama has already been installed.
  useEffect(() => {
    (async () => {
      try {
        const { stdout, stderr } = await runInstaller('--mode=check');
        stderr.trim() && console.error(stderr.trimEnd());
        console.debug(`Installation check: ${stdout.trim()}`);
        if (stdout.trim() === 'true') {
          // Install location exists; just start ollama.
          setInstalled(true);
        }
        setChecked(true);
      } catch (ex) {
        console.error(ex);
        setError(`${ex}`);
      }
    })();
  }, []);

  // Callback for <InstallView> to trigger the install.
  function install() {
    (async () => {
      try {
        console.log(`Installing ollama to...`);
        setInstalling(true);
        const { stdout, stderr } = await runInstaller('--mode=install');
        stderr.trim() && console.error(stderr.trimEnd());
        stdout.trim() && console.debug(stdout.trimEnd());
        setInstalled(true);
      } catch (ex) {
        console.error(ex);
        setError(`${ex}`);
      }
    })();
  }

  // Trigger starting Ollama once it's been installed.
  useEffect(() => {
    (async () => {
      try {
        if (installed) {
          const { stdout, stderr } = await runInstaller('--mode=start');
          stderr.trim() && console.error(stderr.trimEnd());
          stdout.trim() && console.debug(stdout.trimEnd());
          setStarted(true);
        }
      } catch (ex) {
        console.error(ex);
        setError(`${ex}`);
      }
    })();
  }, [installed]);

  return (
    <>{
      !!error ? <div className="error">{error}</div> :
        !installed && !installing && checked ? <InstallView install={install} /> :
          !started ? <LoadingView /> :
            <WebpageFrame />
    }</>
  );
}
