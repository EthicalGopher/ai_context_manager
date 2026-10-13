import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import '@fontsource-variable/urbanist'
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import './index.css'

createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
