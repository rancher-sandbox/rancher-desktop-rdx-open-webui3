import React, { useState, useEffect } from 'react';
import LoadingView from './LoadingView';

const WebpageFrame = () => {
  const [hideIframeView, setHideIframeView] = useState(true);

  useEffect(() => {
    const showIframeViewAfterInterval = async (time: number) => {
      await new Promise(resolve => setTimeout(resolve, time));
      setHideIframeView(false);
    };
    showIframeViewAfterInterval(1000);
  }, []);

  return (
    <>
      {hideIframeView && <LoadingView />}
      <iframe
        src="http://localhost:11500"
        style={{
          position: 'absolute',
          left: '0',
          top: '0',
          width: '100%',
          height: '100%',
          border: 'none',
          opacity: hideIframeView ? 0 : 1,
          transition: 'opacity 0.5s ease-in-out'        
        }}
      />
    </>
  );
};

export default WebpageFrame;
