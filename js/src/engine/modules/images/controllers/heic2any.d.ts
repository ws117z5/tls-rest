declare module "heic2any" {
    interface Heic2AnyOptions {
        blob: Blob;
        toType?: string;   // e.g. "image/jpeg" | "image/png"
        quality?: number;  // 0..1
    }
    const heic2any: (opts: Heic2AnyOptions) => Promise<Blob | Blob[]>;
    export default heic2any;
}