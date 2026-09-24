// Folder grouping for the images list views; hits GET /<module>/folders (images.go's ListFolders) for the full count/preview, not just the loaded page.

import { useEffect, useState } from "react";
import axios from "axios";
import Config from "@engine/Config";
import { ImageRef } from "./images";

export interface FolderSummary {
    folder: string;
    count: number;
    previews: ImageRef[];
}

export function useImageFolders(module: string): { folders: FolderSummary[]; loading: boolean } {
    const [folders, setFolders] = useState<FolderSummary[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let cancelled = false;
        setLoading(true);
        axios
            .get(`${Config.serverURL}${module}/folders`)
            .then((res) => {
                if (!cancelled) setFolders(Array.isArray(res.data) ? res.data : []);
            })
            .catch(() => {
                if (!cancelled) setFolders([]);
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [module]);

    return { folders, loading };
}

// Fetches every image inside one folder (up to the backend's 100-row cap —
// plenty for a deck's worth of browsing) when a folder tile is opened.
export function useFolderImages(module: string, folder: string | null): { images: any[]; loading: boolean } {
    const [images, setImages] = useState<any[]>([]);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (!folder) {
            setImages([]);
            return;
        }
        let cancelled = false;
        setLoading(true);
        axios
            .get(`${Config.serverURL}${module}`, { params: { folder, limit: 100 } })
            .then((res) => {
                if (!cancelled) setImages(res.data?.Data || []);
            })
            .catch(() => {
                if (!cancelled) setImages([]);
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [module, folder]);

    return { images, loading };
}
