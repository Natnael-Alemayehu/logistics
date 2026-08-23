import { File, Directory, Paths } from 'expo-file-system';
import * as ImageManipulator from 'expo-image-manipulator';
import * as Location from 'expo-location';
import { generateUUID } from '../../utils/helpers';
import {
  PODPhoto,
  Coordinates,
  PhotoWithMetadata,
  CompressedPhoto,
} from '../../types/pod';

function getPhotosDirectory(): Directory {
  return new Directory(Paths.document, 'pod_photos');
}

async function ensurePhotosDirectory(): Promise<Directory> {
  const photosDir = getPhotosDirectory();
  if (!photosDir.exists) {
    photosDir.create();
  }
  return photosDir;
}

export async function savePhotoToLocal(
  sourceUri: string,
  filename: string
): Promise<string> {
  const photosDir = await ensurePhotosDirectory();
  const newFile = new File(photosDir, filename);
  
  const sourceFile = new File(sourceUri);
  sourceFile.copy(newFile);
  
  return newFile.uri;
}

export async function getPhotoUri(localPath: string): Promise<string> {
  const file = new File(localPath);
  if (file.exists) {
    return localPath;
  }
  throw new Error(`Photo not found at ${localPath}`);
}

export async function deletePhoto(localPath: string): Promise<void> {
  const file = new File(localPath);
  if (file.exists) {
    file.delete();
  }
}

export async function compressPhoto(
  uri: string,
  maxWidth: number = 800,
  maxHeight: number = 600,
  quality: number = 0.7
): Promise<CompressedPhoto> {
  const result = await ImageManipulator.manipulateAsync(
    uri,
    [{ resize: { width: maxWidth, height: maxHeight } }],
    {
      compress: quality,
      format: ImageManipulator.SaveFormat.JPEG,
    }
  );

  const file = new File(result.uri);
  const fileSize = file.exists ? file.size : 0;

  return {
    uri: result.uri,
    width: result.width,
    height: result.height,
    fileSize,
  };
}

export async function addPhotoMetadata(
  uri: string,
  location: Coordinates | null,
  timestamp: Date
): Promise<PhotoWithMetadata> {
  const file = new File(uri);
  
  let width = 800;
  let height = 600;
  let fileSize = 0;

  if (file.exists) {
    fileSize = file.size;
  }

  return {
    uri,
    width,
    height,
    fileSize,
    latitude: location?.latitude,
    longitude: location?.longitude,
    capturedAt: timestamp.toISOString(),
  };
}

export async function getCurrentLocationForPhoto(): Promise<Coordinates | null> {
  try {
    const { status } = await Location.getForegroundPermissionsAsync();
    if (status !== 'granted') {
      const { status: newStatus } = await Location.requestForegroundPermissionsAsync();
      if (newStatus !== 'granted') {
        return null;
      }
    }

    const location = await Location.getCurrentPositionAsync({
      accuracy: Location.Accuracy.High,
    });

    return {
      latitude: location.coords.latitude,
      longitude: location.coords.longitude,
    };
  } catch {
    return null;
  }
}

export async function getPhotoStorageUsage(): Promise<number> {
  try {
    const photosDir = await ensurePhotosDirectory();
    const items = photosDir.list();
    
    let totalSize = 0;
    for (const item of items) {
      if (item instanceof File) {
        totalSize += item.size;
      }
    }

    return totalSize;
  } catch {
    return 0;
  }
}

export async function cleanupOldPhotos(olderThan: Date): Promise<void> {
  try {
    const photosDir = await ensurePhotosDirectory();
    const items = photosDir.list();

    for (const item of items) {
      if (item instanceof File) {
        const modTime = (item as unknown as { lastModified?: number }).lastModified;
        if (modTime !== undefined && new Date(modTime) < olderThan) {
          item.delete();
        }
      }
    }
  } catch (error) {
    console.error('Error cleaning up old photos:', error);
  }
}

export async function createPODPhoto(
  capturedUri: string,
  location: Coordinates | null,
  timestamp: Date
): Promise<PODPhoto> {
  const compressed = await compressPhoto(capturedUri, 800, 600, 0.7);
  
  const filename = `${generateUUID()}.jpg`;
  const savedUri = await savePhotoToLocal(compressed.uri, filename);
  
  const thumbnail = await compressPhoto(capturedUri, 200, 150, 0.5);
  const thumbnailFilename = `thumb_${filename}`;
  const thumbnailUri = await savePhotoToLocal(thumbnail.uri, thumbnailFilename);

  const metadata = await addPhotoMetadata(savedUri, location, timestamp);

  return {
    id: generateUUID(),
    localUri: savedUri,
    thumbnailUri,
    latitude: metadata.latitude,
    longitude: metadata.longitude,
    capturedAt: metadata.capturedAt,
    fileSize: metadata.fileSize,
    width: metadata.width,
    height: metadata.height,
    uploaded: false,
  };
}

export async function deletePODPhoto(photo: PODPhoto): Promise<void> {
  if (photo.localUri) {
    await deletePhoto(photo.localUri);
  }
  if (photo.thumbnailUri) {
    await deletePhoto(photo.thumbnailUri);
  }
}

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

/**
 * Reads a stored photo back as a base64 data URI for upload.
 *
 * The sync payload carries image bytes, not device paths: a localUri is
 * meaningful only on the handset that captured it, so sending one produced
 * proof-of-delivery records pointing at files nobody else could open.
 *
 * Photos are already compressed to 800x600 at quality 0.7 by createPODPhoto, so
 * the encoded form stays small enough for a poor connection.
 */
export async function readPhotoAsBase64(uri: string): Promise<string> {
  const file = new File(uri);
  if (!file.exists) {
    throw new Error(`photo no longer exists on device: ${uri}`);
  }

  return `data:image/jpeg;base64,${await file.base64()}`;
}
