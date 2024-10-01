import { useState, useEffect } from 'react';
import { createDockerDesktopClient } from '@docker/extension-api-client';
import WebpageFrame from './WebpageFrame';
import InstallView from './InstallView';
import LoadingView from './LoadingView';

const ddClient = createDockerDesktopClient();

export function App() {
  const [error, setError] = useState('');
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
        const { stdout, stderr } = await runInstaller('--mode=locate');
        stderr.trim() && console.error(stderr.trimEnd());
        console.debug(`Got install location: ${stdout.trim() || '<none>'}`);
        if (stdout.trim()) {
          // Install location exists; just start ollama.
          setInstalled(true);
        }
      } catch (ex) {
        console.error(ex);
        setError(`${ex}`);
      }
    })();
  });

  // Callback for <InstallView> to trigger the install.
  function install(installLocation: string | undefined) {
    (async () => {
      try {
        console.log(`Installing ollama to ${installLocation ?? '<default>'}...`);
        setInstalling(true);
        const { stdout, stderr } = await runInstaller('--mode=install', `--install-path=${installLocation ?? ''}`);
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
        !installed && !installing ? <InstallView install={install} /> :
          !started ? <LoadingView /> :
            <WebpageFrame />
    }</>
  );
}
