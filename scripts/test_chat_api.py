from openai import OpenAI


def main() -> None:
    client = OpenAI(
        api_key="sk-local-demo",
        base_url="http://localhost:8080/v1",
    )

    response = client.chat.completions.create(
        model="local-llama",
        messages=[
            {"role": "user", "content": "Explain LLM gateways."},
        ],
    )

    print(response.choices[0].message.content)


if __name__ == "__main__":
    main()
