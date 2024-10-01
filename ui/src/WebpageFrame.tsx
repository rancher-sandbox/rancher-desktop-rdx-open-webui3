import React from 'react';

const WebpageFrame = () => {
  return (
    <iframe
      src="http://localhost:3000"
      style={{
        position: 'absolute',
        left: '0',
        top: '0',
        width: '100%',
        height: '100%',
        border: 'none', 
      }}
    />
  );
};

export default WebpageFrame;
