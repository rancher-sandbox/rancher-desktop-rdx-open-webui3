import React from 'react';

const WebpageFrame = () => {
  return (
    <iframe
      src="http://localhost:3000"
      style={{
        width: '100%',
        height: '100vh', 
        border: 'none', 
      }}
    />
  );
};

export default WebpageFrame;
