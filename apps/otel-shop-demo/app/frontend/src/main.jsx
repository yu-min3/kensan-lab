import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import { initTelemetry } from './telemetry/otel';
import './index.css';

// Initialize OpenTelemetry before rendering the app
initTelemetry();

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
