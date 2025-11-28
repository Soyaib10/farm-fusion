import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.preprocessing import LabelEncoder
import joblib
import os

def train_crop_model():
    print("Training crop recommendation model...")
    df = pd.read_csv('data/crop_recommendation.csv')
    
    X = df[['N', 'P', 'K', 'temperature', 'humidity', 'ph', 'rainfall']]
    y = df['label']
    
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)
    
    model = RandomForestClassifier(n_estimators=100, random_state=42)
    model.fit(X_train, y_train)
    
    accuracy = model.score(X_test, y_test)
    print(f"Crop model accuracy: {accuracy:.2%}")
    
    os.makedirs('app/models', exist_ok=True)
    joblib.dump(model, 'app/models/crop_model.pkl')
    print("Crop model saved to app/models/crop_model.pkl")

def train_fertilizer_model():
    print("\nTraining fertilizer recommendation model...")
    df = pd.read_csv('data/fertilizer_recommendation.csv')
    
    # Encode categorical features
    le_soil = LabelEncoder()
    le_crop = LabelEncoder()
    
    df['Soil Type'] = le_soil.fit_transform(df['Soil Type'])
    df['Crop Type'] = le_crop.fit_transform(df['Crop Type'])
    
    X = df[['Temparature', 'Humidity', 'Soil Moisture', 'Soil Type', 'Crop Type', 'Nitrogen', 'Potassium', 'Phosphorous']]
    y = df['Fertilizer Name']
    
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)
    
    model = RandomForestClassifier(n_estimators=100, random_state=42)
    model.fit(X_train, y_train)
    
    accuracy = model.score(X_test, y_test)
    print(f"Fertilizer model accuracy: {accuracy:.2%}")
    
    joblib.dump(model, 'app/models/fertilizer_model.pkl')
    joblib.dump(le_soil, 'app/models/soil_encoder.pkl')
    joblib.dump(le_crop, 'app/models/crop_encoder.pkl')
    print("Fertilizer model and encoders saved to app/models/")

if __name__ == "__main__":
    train_crop_model()
    train_fertilizer_model()
    print("\n✓ All models trained successfully!")
