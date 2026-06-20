import { createOpenAI } from '@ai-sdk/openai';
import { streamText } from 'ai';

// Allow streaming responses up to 30 seconds
export const maxDuration = 30;

export async function POST(req: Request) {
  const { messages } = await req.json();

  // Connect to omniagent OpenAI-compatible API
  const omniagent = createOpenAI({
    baseURL: process.env.OMNIAGENT_BASE_URL || 'http://localhost:8080/v1',
    apiKey: process.env.OMNIAGENT_API_KEY || 'dummy-key',
  });

  const result = await streamText({
    model: omniagent('omniagent'),
    messages,
  });

  return result.toTextStreamResponse();
}
