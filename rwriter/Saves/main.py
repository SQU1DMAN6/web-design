import os
import json
import time
import requests
import asyncio
import subprocess
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
from ddgs import DDGS
import tempfile
import shutil
from typing import Dict, Any
from bs4 import BeautifulSoup
import re

USERDATA_DIR = os.path.expanduser("~/RWRiter-userdata/")
OLLAMA_URL = "http://127.0.0.1:11434/api/generate"

app = FastAPI()

# --- CORS ---
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- Health check ---
@app.get("/healthz")
async def healthz():
    return {"ok": True}

# --- List chats ---
@app.get("/chats/{username}")
async def list_chats(username: str):
    user_dir = os.path.join(USERDATA_DIR, username)
    os.makedirs(user_dir, exist_ok=True)
    chats = [d for d in os.listdir(user_dir) if os.path.isdir(os.path.join(user_dir, d))]
    chats.sort()
    return {"chats": chats}

# --- Create new chat ---
@app.post("/chats/{username}/new")
async def new_chat(username: str):
    user_dir = os.path.join(USERDATA_DIR, username)
    os.makedirs(user_dir, exist_ok=True)
    chat_id = f"chat_{int(time.time() * 1000)}"
    chat_dir = os.path.join(user_dir, chat_id)
    os.makedirs(chat_dir)
    with open(os.path.join(chat_dir, "data.json"), "w", encoding="utf-8") as f:
        json.dump([], f, ensure_ascii=False, indent=2)
    return {"chat_id": chat_id}

# --- Delete chat ---
@app.delete("/chats/{username}/{chatname}")
async def delete_chat(username: str, chatname: str):
    chat_dir = os.path.join(USERDATA_DIR, username, chatname)
    if not os.path.exists(chat_dir):
        return {"ok": False, "error": "Chat not found"}
    try:
        shutil.rmtree(chat_dir)
        return {"ok": True}
    except Exception as e:
        return {"ok": False, "error": str(e)}

# --- Load chat history ---
@app.get("/session/{username}/{chatname}")
async def get_session(username: str, chatname: str):
    chat_file = os.path.join(USERDATA_DIR, username, chatname, "data.json")
    if not os.path.exists(chat_file):
        return {"history": []}
    try:
        with open(chat_file, "r", encoding="utf-8") as f:
            history = json.load(f)
        return {"history": history}
    except Exception as e:
        return {"history": [], "error": str(e)}

# --- DuckDuckGo search wrapper ---
def run_search(query: str, max_results=5) -> str:
    """Run a search query and return formatted results."""
    try:
        results = []
        with DDGS() as ddgs:
            for r in ddgs.text(query, safesearch="Moderate", max_results=max_results):
                title = r.get('title', '').strip()
                url = r.get('url', '').strip()
                body = r.get('body', '').strip()[:200]  # Truncate body
                results.append(f"Title: {title}\nURL: {url}\nSummary: {body}\n")

        return "\n".join(results) if results else "No results found."
    except Exception as e:
        return f"Search error: {str(e)}"

# --- Web navigation and DOM scraping ---
def scrape_webpage(url: str, extract_type: str = "content") -> str:
    """Scrape a webpage and extract specific content based on type."""
    try:
        # Add protocol if missing
        if not url.startswith(('http://', 'https://')):
            url = 'https://' + url
        
        # Set headers to mimic a real browser
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'en-US,en;q=0.5',
            'Accept-Encoding': 'gzip, deflate',
            'Connection': 'keep-alive',
        }
        
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        
        soup = BeautifulSoup(response.content, 'html.parser')
        
        # Remove script and style elements
        for script in soup(["script", "style"]):
            script.decompose()
        
        if extract_type == "title":
            title = soup.find('title')
            return title.get_text().strip() if title else "No title found"
        
        elif extract_type == "headings":
            headings = []
            for i in range(1, 7):  # h1 to h6
                for heading in soup.find_all(f'h{i}'):
                    headings.append(f"H{i}: {heading.get_text().strip()}")
            return "\n".join(headings) if headings else "No headings found"
        
        elif extract_type == "links":
            links = []
            for link in soup.find_all('a', href=True):
                href = link['href']
                text = link.get_text().strip()
                if text and href:
                    # Convert relative URLs to absolute
                    if href.startswith('/'):
                        from urllib.parse import urljoin
                        href = urljoin(url, href)
                    links.append(f"{text}: {href}")
            return "\n".join(links[:20]) if links else "No links found"  # Limit to 20 links
        
        elif extract_type == "images":
            images = []
            for img in soup.find_all('img', src=True):
                src = img['src']
                alt = img.get('alt', '')
                # Convert relative URLs to absolute
                if src.startswith('/'):
                    from urllib.parse import urljoin
                    src = urljoin(url, src)
                images.append(f"Image: {alt or 'No alt text'} - {src}")
            return "\n".join(images[:10]) if images else "No images found"  # Limit to 10 images
        
        elif extract_type == "tables":
            tables = []
            for table in soup.find_all('table'):
                table_data = []
                for row in table.find_all('tr'):
                    cells = [cell.get_text().strip() for cell in row.find_all(['td', 'th'])]
                    if cells:
                        table_data.append(" | ".join(cells))
                if table_data:
                    tables.append("\n".join(table_data))
            return "\n\n".join(tables) if tables else "No tables found"
        
        else:  # Default: extract main content
            # Try to find main content areas
            main_content = soup.find('main') or soup.find('article') or soup.find('div', class_=re.compile(r'content|main|body'))
            
            if main_content:
                text = main_content.get_text()
            else:
                text = soup.get_text()
            
            # Clean up the text
            lines = (line.strip() for line in text.splitlines())
            chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
            text = ' '.join(chunk for chunk in chunks if chunk)
            
            # Limit length
            if len(text) > 3000:
                text = text[:3000] + "... [truncated]"
            
            return text if text else "No content found"
    
    except requests.RequestException as e:
        return f"Error fetching webpage: {str(e)}"
    except Exception as e:
        return f"Error scraping webpage: {str(e)}"

# --- Python execution wrapper with sandboxing ---
def execute_python(code: str) -> str:
    """Execute Python code safely in a sandboxed environment."""
    try:
        # Security checks - block dangerous operations
        dangerous_patterns = [
            'import os', 'import sys', 'import subprocess', 'import shutil',
            'import socket', 'import urllib', 'import requests', 'import http',
            'import ftplib', 'import smtplib', 'import telnetlib',
            'open(', 'file(', 'exec(', 'eval(', 'compile(',
            '__import__', 'getattr', 'setattr', 'delattr',
            'input(', 'raw_input(', 'exit(', 'quit(',
            'os.', 'sys.', 'subprocess.', 'shutil.',
            'socket.', 'urllib.', 'requests.', 'http.',
            'ftplib.', 'smtplib.', 'telnetlib.'
        ]
        
        code_lower = code.lower()
        for pattern in dangerous_patterns:
            if pattern in code_lower:
                return f"Error: Blocked potentially dangerous operation: {pattern}"
        
        # Create a temporary directory for sandboxed execution
        with tempfile.TemporaryDirectory() as temp_dir:
            # Create the Python file in the sandbox directory
            python_file = os.path.join(temp_dir, "code.py")
            with open(python_file, 'w') as f:
                f.write(code)
            
            # Create a restricted environment
            restricted_env = os.environ.copy()
            restricted_env['PYTHONPATH'] = temp_dir
            restricted_env['PATH'] = '/usr/bin:/bin'  # Minimal PATH
            
            # Execute with additional security measures
            result = subprocess.run(
                ['python3', '-B', '-E', '-s', python_file],  # -B: no .pyc, -E: ignore env, -s: no user site
                capture_output=True,
                text=True,
                timeout=15,  # 15-second timeout
                cwd=temp_dir,  # Execute in sandbox directory
                env=restricted_env,
                preexec_fn=None  # Don't set preexec_fn to avoid permission issues
            )
            
            # Return output or error
            if result.returncode == 0:
                output = result.stdout.strip()
                return output if output else "Code executed successfully (no output)"
            else:
                error_msg = result.stderr.strip()
                # Sanitize error messages to avoid exposing system paths
                error_msg = error_msg.replace(temp_dir, "[SANDBOX]")
                return f"Error: {error_msg}"

    except subprocess.TimeoutExpired:
        return "Error: Code execution timed out (15 seconds)"
    except PermissionError:
        return "Error: Permission denied - sandbox security restriction"
    except Exception as e:
        return f"Execution error: {str(e)}"

# --- Action handlers ---
def handle_search_action(action_obj: Dict[str, Any]) -> str:
    """Handle search action and return results."""
    query = action_obj.get("input", "").strip()
    if not query:
        return "Error: No search query provided"

    print(f"[SEARCH] Query: {query}")
    results = run_search(query)
    print(f"[SEARCH] Results: {results[:200]}...")
    return results

def handle_python_action(action_obj: Dict[str, Any]) -> str:
    """Handle Python execution action and return results."""
    code = action_obj.get("input", "").strip()
    if not code:
        return "Error: No Python code provided"

    print(f"[PYTHON] Executing code:\n{code}")
    result = execute_python(code)
    print(f"[PYTHON] Result: {result}")
    return result

def handle_web_action(action_obj: Dict[str, Any]) -> str:
    """Handle web navigation and scraping action."""
    url = action_obj.get("input", "").strip()
    extract_type = action_obj.get("extract_type", "content").strip()
    
    if not url:
        return "Error: No URL provided"
    
    print(f"[WEB] Scraping URL: {url}, Type: {extract_type}")
    result = scrape_webpage(url, extract_type)
    print(f"[WEB] Result: {result}...")
    return result

# --- Improved JSON parsing ---
def extract_json_from_text(text: str) -> Dict[str, Any] | None:
    """Try to extract JSON from text, ensuring complete JSON before parsing."""
    text = text.strip()

    # Try direct JSON parsing first
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass

    # Look for complete JSON structures only
    if text.startswith('{') and text.endswith('}'):
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            pass

    # Look for complete JSON within the text - only return if it's a complete, valid JSON
    import re
    json_pattern = r'\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}'
    matches = re.findall(json_pattern, text)

    for match in matches:
        try:
            parsed = json.loads(match)
            # Only return if it's a complete action JSON with required fields
            if isinstance(parsed, dict) and "action" in parsed and "input" in parsed:
                return parsed
        except json.JSONDecodeError:
            continue

    return None

def is_complete_json(text: str) -> bool:
    """Check if the text contains a complete, valid JSON action."""
    return extract_json_from_text(text) is not None

# --- Streaming chat endpoint ---
@app.post("/chat/{username}/{chatname}")
async def chat(username: str, chatname: str, request: Request):
    data = await request.json()
    prompt = data.get("prompt", "").strip()
    if not prompt:
        raise HTTPException(status_code=400, detail="No prompt provided")

    chat_file = os.path.join(USERDATA_DIR, username, chatname, "data.json")
    os.makedirs(os.path.dirname(chat_file), exist_ok=True)

    # Load history
    history = []
    if os.path.exists(chat_file):
        try:
            with open(chat_file, "r", encoding="utf-8") as f:
                history = json.load(f)
        except Exception as e:
            print(f"Error loading history: {e}")

    # Append user message
    history.append({"role": "user", "content": prompt})

    # Build context with better formatting
    context_messages = []
    for msg in history[-10:]:  # Keep last 10 messages for context
        role = msg['role'].upper()
        content = msg['content']
        context_messages.append(f"[{role}] {content}")

    context_text = "\n".join(context_messages)

    def event_stream():
            final_response = ""
            last_keepalive = time.time()

            def maybe_keepalive():
                nonlocal last_keepalive
                now = time.time()
                if now - last_keepalive >= 15:
                    last_keepalive = now
                    # minimal keepalive: a single newline
                    return "\n"
                return None

            try:
                with requests.post(
                    OLLAMA_URL,
                    json={
                        "model": "RWRiter:latest",
                        "prompt": context_text,
                        "stream": True
                    },
                    stream=True,
                    timeout=600
                ) as ai_res:

                    if not ai_res.ok:
                        yield f"[ERROR: AI status {ai_res.status_code}]"
                        return

                    # send an initial flush to open the stream quickly
                    yield "\n"

                    for chunk in ai_res.iter_content(chunk_size=512):
                        if not chunk:
                            keep = maybe_keepalive()
                            if keep:
                                yield keep
                            continue
                        text_piece = chunk.decode("utf-8", errors='ignore')
                        if text_piece:
                            final_response += text_piece
                            yield text_piece

                    # final keepalive just in case of buffering
                    keep = maybe_keepalive()
                    if keep:
                        yield keep

            except requests.RequestException as e:
                yield f"⚠ Connection error: {e}"
            except Exception as e:
                yield f"⚠ Unexpected error: {e}"

            try:
                history.append({"role": "bot", "content": final_response})
                with open(chat_file, "w", encoding="utf-8") as f:
                    json.dump(history, f, ensure_ascii=False, indent=2)
            except Exception as e:
                print(f"Error saving chat: {e}")

    return StreamingResponse(event_stream(), media_type="text/plain")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8080, reload=True)
