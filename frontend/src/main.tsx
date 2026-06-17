import React from 'react'
import {createRoot} from 'react-dom/client'
import "./index.css"
import App from './App'
import { getApiBase } from './_lib/wails'

const container = document.getElementById('root')

const root = createRoot(container!)

function render() {
    root.render(
        <React.StrictMode>
            <App/>
        </React.StrictMode>
    )
}

// Resolve the API base (from the Wails binding when present, else the fallback)
// once before the first render so every synchronous `apiBaseSync()` read — the
// REST client and the SSE stream — uses the correct loopback base/port. Render
// regardless of success so browser/dev (no binding) still boots on the fallback.
getApiBase().finally(render)
