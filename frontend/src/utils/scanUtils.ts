import { InputSource } from "../types";

export async function initiateScan(inputSource: InputSource, apiBaseUrl: string) {
    try {
      const response = await fetch(
        `${apiBaseUrl}/api/scan`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({inputSource}),
        }
      );
  
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
  
      console.log("Scan initiated successfully");
      return true;
    } catch (error) {
      console.error("Error initiating scan:", error);
      return false;
    }
  }