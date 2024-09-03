import { useState, useEffect } from 'react';
import FadeLoader from "react-spinners/FadeLoader";
import { createDockerDesktopClient } from '@docker/extension-api-client';
import WebpageFrame from './WebpageFrame';
import './LoaderComponent.css';

const client = createDockerDesktopClient();

function useDockerDesktopClient() {
  return client;
}

export function App() {
  const [loading, setLoading] = useState(true);
  const ddClient = useDockerDesktopClient();

  useEffect(() => {
    const run = async () => {
      let binary = "installer";
      if (ddClient.host.platform === 'win32') {
        binary += ".exe";
      }

      await ddClient.extension.host?.cli.exec(binary, []);
      setLoading(false);
    };
    run();
  }, [ddClient]);

  return (
    <>  
        {loading ? 
        <div className="loader-container">
            <FadeLoader 
              loading={loading}
              color="#265277" 
            />
        </div> :
        <WebpageFrame />
        }
    </>
  );
}
